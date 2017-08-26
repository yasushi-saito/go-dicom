package dicom_test

import (
	"encoding/binary"
	"github.com/yasushi-saito/go-dicom"
	"testing"
)

func testEncodeDataElement(t *testing.T, bo binary.ByteOrder, implicit dicom.IsImplicitVR) {
	// Encode two scalar elements.
	e := dicom.NewEncoder(bo, implicit)
	var values []interface{}
	values = append(values, string("FooHah"))
	dicom.EncodeDataElement(e, &dicom.DicomElement{
		Tag:   dicom.Tag{0x0018, 0x9755},
		Value: values})
	values = nil
	values = append(values, uint32(1234))
	values = append(values, uint32(2345))
	dicom.EncodeDataElement(e, &dicom.DicomElement{
		Tag:   dicom.Tag{0x0020, 0x9057},
		Value: values})

	data, err := e.Finish()
	if err != nil {
		t.Error(err)
	}

	// Read them back.
	d := dicom.NewBytesDecoder(data, bo, implicit)
	elem0 := dicom.ReadDataElement(d)
	if d.Error()!=nil{
		t.Fatal(d.Error())
	}
	tag := dicom.Tag{0x18, 0x9755}
	if elem0.Tag != tag {
		t.Error("Bad tag", elem0)
	}
	if len(elem0.Value) != 1 {
		t.Error("Bad value", elem0)
	}
	if elem0.Value[0].(string) != "FooHah" {
		t.Error("Bad value", elem0)
	}

	tag = dicom.Tag{Group: 0x20, Element: 0x9057}
	elem1 := dicom.ReadDataElement(d)
	if d.Error()!=nil{
		t.Fatal(d.Error())
	}
	if elem1.Tag != tag {
		t.Error("Bad tag")
	}
	if len(elem1.Value) != 2 {
		t.Error("Bad value", elem1)
	}
	if elem1.Value[0].(uint32) != 1234 {
		t.Error("Bad value", elem1)
	}
	if elem1.Value[1].(uint32) != 2345 {
		t.Error("Bad value", elem1)
	}
	if err := d.Finish(); err != nil {
		t.Error(err)
	}
}

func TestEncodeDataElementImplicit(t *testing.T) {
	// testEncodeDataElement(t, binary.LittleEndian, dicom.ImplicitVR)
}

func TestEncodeDataElementExplicit(t *testing.T) {
	testEncodeDataElement(t, binary.LittleEndian, dicom.ExplicitVR)
}

func TestEncodeDataElementBigEndianExplicit(t *testing.T) {
	testEncodeDataElement(t, binary.BigEndian, dicom.ExplicitVR)
}
