// DICOM file parser. Example:
//
//   package main
//
//   import (
// 	"fmt"
// 	"github.com/gillesdemey/go-dicom"
// 	"os"
//   )
//
//   func main() {
//     in, err := os.Open("myfile.dcm")
//     st, err := in.Stat()
//     data, err := dicom.Parse(in, st.Size())
//     if err != nil {
//         panic(err)
//     }
//     for _, elem := range(data.Elements) {
//         fmt.Printf("%+v\n", elem)
//     }
//   }
package dicom

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// UID prefix provided by https://www.medicalconnections.co.uk/Free_UID
const DefaultImplementationClassUIDPrefix = "1.2.826.0.1.3680043.9.7133"

var DefaultImplementationClassUID = DefaultImplementationClassUIDPrefix + ".1.1"

const DefaultImplementationVersionName = "GODICOM_1_1"

// DicomFile represents result of parsing one DICOM file.
type DicomFile struct {
	Elements []DicomElement
}

// ParseBytes(buf) is shorthand for Parse(bytes.NewBuffer(buf), len(buf)).
func ParseBytes(data []byte) (*DicomFile, error) {
	return Parse(bytes.NewBuffer(data), int64(len(data)))
}

// Parse up to "bytes" from "io" as DICOM file. Returns a DICOM file struct
//
// TODO(saito) Get rid of the "bytes" argument. Detect io.EOF instead.
func Parse(in io.Reader, bytes int64) (*DicomFile, error) {
	// buffer := newDicomBuffer(buff) //*di.Bytes)
	buffer := NewDecoder(in,
		bytes,
		binary.LittleEndian,
		ExplicitVR)

	metaElems := ParseFileHeader(buffer)
	if buffer.Error() != nil {
		return nil, buffer.Error()
	}
	file := &DicomFile{Elements: metaElems}
	elem, err := file.LookupElement("TransferSyntaxUID")
	if err != nil {
		return nil, err
	}
	transferSyntaxUID, err := GetString(*elem)
	if err != nil {
		return nil, err
	}
	// read endianness and explicit VR
	endianess, implicit, err := ParseTransferSyntaxUID(transferSyntaxUID)
	if err != nil {
		return nil, err
	}
	// modify buffer according to new TransferSyntaxUID
	buffer.bo = endianess
	buffer.implicit = implicit

	for buffer.Len() != 0 && buffer.Error() == nil {
		elem := ReadDataElement(buffer)
		if buffer.Error() != nil {
			break
		}
		file.Elements = append(file.Elements, *elem)
	}
	return file, buffer.Finish()
}

// Consume the DICOM magic header and metadata elements from a Dicom
// file. Errors are reported through d.Error().
func ParseFileHeader(d *Decoder) []DicomElement {
	d.PushTransferSyntax(binary.LittleEndian, ExplicitVR)
	defer d.PopTransferSyntax()
	d.Skip(128) // skip preamble

	// check for magic word
	if s := d.DecodeString(4); s != "DICM" {
		d.SetError(errors.New("Keyword 'DICM' not found in the header"))
		return nil
	}

	// (0002,0000) MetaElementGroupLength
	metaElem := ReadDataElement(d)
	if d.Error() != nil {
		return nil
	}
	if metaElem.Tag != TagMetaElementGroupLength {
		d.SetError(fmt.Errorf("MetaElementGroupLength not found; insteadfound %s", metaElem.Tag.String()))
	}
	metaLength, err := GetUInt32(*metaElem)
	if err != nil {
		d.SetError(fmt.Errorf("Failed to read uint32 in MetaElementGroupLength"))
		return nil
	}
	if d.Len() <= 0 {
		d.SetError(fmt.Errorf("No data element found"))
		return nil
	}
	metaElems := []DicomElement{*metaElem}

	// Read meta tags
	d.PushLimit(int64(metaLength))
	defer d.PopLimit()
	for d.Len() > 0 && d.Error() == nil {
		elem := ReadDataElement(d)
		metaElems = append(metaElems, *elem)
	}
	return metaElems
}

// Inverse of ParseFileHeader. Errors are reported via e.Error()
func WriteFileHeader(e *Encoder,
	transferSyntaxUID string,
	sopClassUID string,
	sopInstanceUID string) {
	e.PushTransferSyntax(binary.LittleEndian, ExplicitVR)
	defer e.PopTransferSyntax()
	encodeSingleValue := func(encoder *Encoder, tag Tag, v interface{}) {
		elem := DicomElement{
			Tag:   tag,
			Vr:    "", // autodetect
			Vl:    1,
			Value: []interface{}{v},
		}
		EncodeDataElement(encoder, &elem)
	}

	// Encode the meta info first.
	subEncoder := NewEncoder(binary.LittleEndian, ExplicitVR)
	encodeSingleValue(subEncoder, TagFileMetaInformationVersion, []byte("0 1"))
	encodeSingleValue(subEncoder, TagTransferSyntaxUID, transferSyntaxUID)
	encodeSingleValue(subEncoder, TagMediaStorageSOPClassUID, sopClassUID)
	encodeSingleValue(subEncoder, TagMediaStorageSOPInstanceUID, sopInstanceUID)
	encodeSingleValue(subEncoder, TagImplementationClassUID, DefaultImplementationClassUID)
	encodeSingleValue(subEncoder, TagImplementationVersionName, DefaultImplementationVersionName)
	// TODO(saito) add more
	metaBytes, err := subEncoder.Finish()
	if err != nil {
		e.SetError(err)
		return
	}

	e.EncodeZeros(128)
	e.EncodeString("DICM")
	encodeSingleValue(e, TagMetaElementGroupLength, uint32(len(metaBytes)))
	e.EncodeBytes(metaBytes)
}

func doassert(x bool) {
	if !x {
		panic("doassert")
	}
}

// Finds the SyntaxTrasnferUID and returns the endianess and implicit VR for the file
func (file *DicomFile) getTransferSyntax() (binary.ByteOrder, IsImplicitVR, error) {
	var err error

	elem, err := file.LookupElement("TransferSyntaxUID")
	if err != nil {
		return nil, UnknownVR, err
	}
	ts, err := GetString(*elem)
	if err != nil {
		return nil, UnknownVR, err
	}
	return ParseTransferSyntaxUID(ts)
}

// LookupElementByName finds an element with the given DicomElement.Name in
// "elems" If not found, returns an error.
func LookupElementByName(elems []DicomElement, name string) (*DicomElement, error) {
	t, err := LookupTagByName(name)
	if err != nil {
		return nil, err
	}
	for _, elem := range elems {
		if elem.Tag == t.Tag {
			return &elem, nil
		}
	}
	return nil, fmt.Errorf("Could not find element named '%s' in dicom file", name)
}

// LookupElementByTag finds an element with the given DicomElement.Tag in
// "elems" If not found, returns an error.
func LookupElementByTag(elems []DicomElement, tag Tag) (*DicomElement, error) {
	for _, elem := range elems {
		if elem.Tag == tag {
			return &elem, nil
		}
	}
	return nil, fmt.Errorf("Could not find element with tag %s in dicom file",
		tag.String())
}

// Lookup a tag by name. Depraceted. Use
// LookupElementByName(file.Elemements, name) or
// LookupElementByTag(file.Elemements, tag) instead
func (file *DicomFile) LookupElement(name string) (*DicomElement, error) {
	return LookupElementByName(file.Elements, name)
}
