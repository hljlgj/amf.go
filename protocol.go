package amf

import (
	"encoding/binary"
	"fmt"
	"os"
	"reflect"
)

type Reader interface {
	Read(p []byte) (n int, err os.Error)
}

type Writer interface {
	Write(p []byte) (n int, err os.Error)
}

type AvmObject struct {
	class         *AvmClass
	staticFields  []interface{}
	dynamicFields map[string]interface{}
}

type AvmClass struct {
	name           string
	externalizable bool
	dynamic        bool
	properties     []string
}

// An "Array" in AVM land is actually stored as a combination of an array and
// a dictionary.
type AvmArray struct {
	elements []interface{}
	fields   map[string]interface{}
}

// * Public functions *

// Read an AMF3 value from the stream.
func ReadValueAmf3(stream Reader) (interface{}, os.Error) {
	cxt := &Decoder{}
	cxt.AmfVersion = 3
    cxt.stream = stream
	result := cxt.ReadValueAmf3()
	return result, cxt.decodeError
}

func WriteValueAmf3(stream Writer, value interface{}) os.Error {
    return writeValueAmf3(stream, value)
}

// Type markers
const (
	amf0_numberType        = 0
	amf0_booleanType       = 1
	amf0_stringType        = 2
	amf0_objectType        = 3
	amf0_movieClipType     = 4
	amf0_nullType          = 5
	amf0_undefinedType     = 6
	amf0_referenceType     = 7
	amf0_ecmaArrayType     = 8
	amf0_objectEndType     = 9
	amf0_strictArrayType   = 10
	amf0_dateType          = 11
	amf0_longStringType    = 12
	amf0_unsupporedType    = 13
	amf0_recordsetType     = 14
	amf0_xmlObjectType     = 15
	amf0_typedObjectType   = 16
	amf0_avmPlusObjectType = 17

	amf3_undefinedType  = 0
	amf3_nullType       = 1
	amf3_falseType      = 2
	amf3_trueType       = 3
	amf3_integerType    = 4
	amf3_doubleType     = 5
	amf3_stringType     = 6
	amf3_xmlType        = 7
	amf3_dateType       = 8
	amf3_arrayType      = 9
	amf3_objectType     = 10
	amf3_avmPlusXmlType = 11
	amf3_byteArrayType  = 12
)

type Decoder struct {
    stream Reader

	AmfVersion uint16

	// AMF3 messages can include references to previously-unpacked objects. These
	// tables hang on to objects for later use.
	stringTable []string
	classTable  []*AvmClass
	objectTable []interface{}

	decodeError os.Error
}

func NewDecoder(stream Reader, amfVersion uint16) *Decoder {
    decoder := &Decoder{}
    decoder.stream = stream
    decoder.AmfVersion = amfVersion
    return decoder
}

func (cxt *Decoder) useAmf3() bool {
	return cxt.AmfVersion == 3
}
func (cxt *Decoder) saveError(err os.Error) {
	if err == nil {
		return
	}
	if cxt.decodeError != nil {
		fmt.Println("warning: duplicate errors on Decoder")
	} else {
		cxt.decodeError = err
	}
}
func (cxt *Decoder) errored() bool {
	return cxt.decodeError != nil
}
func (cxt *Decoder) storeObjectInTable(obj interface{}) {
	cxt.objectTable = append(cxt.objectTable, obj)
}

type EncodeContext struct {
}


// Helper functions.
func (cxt *Decoder) ReadByte() uint8 {
	buf := make([]byte, 1)
	_, err := cxt.stream.Read(buf)
	cxt.saveError(err)
	return buf[0]
}
func (cxt *Decoder) ReadUint8() uint8 {
	var value uint8
	err := binary.Read(cxt.stream, binary.BigEndian, &value)
	cxt.saveError(err)
	return value
}
func (cxt *Decoder) ReadUint16() uint16 {
	var value uint16
	err := binary.Read(cxt.stream, binary.BigEndian, &value)
	cxt.saveError(err)
	return value
}
func writeUint16(stream Writer, value uint16) os.Error {
	return binary.Write(stream, binary.BigEndian, &value)
}
func (cxt *Decoder) ReadUint32() uint32 {
	var value uint32
	err := binary.Read(cxt.stream, binary.BigEndian, &value)
	cxt.saveError(err)
	return value
}
func writeUint32(stream Writer, value uint32) os.Error {
	return binary.Write(stream, binary.BigEndian, &value)
}
func (cxt *Decoder) ReadFloat64() float64 {
	var value float64
	err := binary.Read(cxt.stream, binary.BigEndian, &value)
	cxt.saveError(err)
	return value
}
func WriteFloat64(stream Writer, value float64) os.Error {
	return binary.Write(stream, binary.BigEndian, &value)
}
func (cxt *Decoder) ReadString() string {
	length := int(cxt.ReadUint16())
	if cxt.errored() {
		return ""
	}
	return cxt.ReadStringKnownLength(length)
}

func (cxt *Decoder) ReadStringKnownLength(length int) string {
	data := make([]byte, length)
	n, err := cxt.stream.Read(data)
	if n < length {
		cxt.saveError(os.NewError(fmt.Sprintf(
			"Not enough bytes in ReadStringKnownLength (expected %d, found %d)", length, n)))
		return ""
	}
	cxt.saveError(err)
	return string(data)
}
func WriteString(stream Writer, str string) os.Error {
	binary.Write(stream, binary.BigEndian, uint16(len(str)))
	_, err := stream.Write([]byte(str))
	return err
}
func writeByte(stream Writer, b uint8) os.Error {
	return binary.Write(stream, binary.BigEndian, b)
}
func writeBool(stream Writer, b bool) {
	val := 0x0
	if b {
		val = 0xff
	}
	binary.Write(stream, binary.BigEndian, uint8(val))
}

func WriteInt32(stream Writer, val int32) os.Error {
	return binary.Write(stream, binary.BigEndian, val)
}

// Read a 29-bit compact encoded integer (as defined in AVM3)
func (cxt *Decoder) ReadUint29() uint32 {
	var result uint32 = 0
	for i := 0; i < 4; i++ {
		b := cxt.ReadByte()

		if cxt.errored() {
			return 0
		}

		if i == 3 {
			// Last byte does not use the special 0x80 bit.
			result = (result << 8) + uint32(b)
		} else {
			result = (result << 7) + (uint32(b) & 0x7f)
		}

		if (b & 0x80) == 0 {
			break
		}
	}
	return result
}

func WriteUint29(stream Writer, value uint32) os.Error {

	// Make sure the value is only 29 bits.
	remainder := value & 0x1fffffff
	if remainder != value {
		fmt.Println("warning: WriteUint29 received a value that does not fit in 29 bits")
	}

	if remainder > 0x1fffff {
		writeByte(stream, uint8(remainder>>22)&0x7f+0x80)
		writeByte(stream, uint8(remainder>>15)&0x7f+0x80)
		writeByte(stream, uint8(remainder>>8)&0x7f+0x80)
		writeByte(stream, uint8(remainder>>0)&0xff)
	} else if remainder > 0x3fff {
		writeByte(stream, uint8(remainder>>14)&0x7f+0x80)
		writeByte(stream, uint8(remainder>>7)&0x7f+0x80)
		writeByte(stream, uint8(remainder>>0)&0x7f)
	} else if remainder > 0x7f {
		writeByte(stream, uint8(remainder>>7)&0x7f+0x80)
		writeByte(stream, uint8(remainder>>0)&0x7f)
	} else {
		writeByte(stream, uint8(remainder))
	}

	return nil
}

func (cxt *Decoder) readStringAmf3() string {
	ref := cxt.ReadUint29()

	if cxt.errored() {
		return ""
	}

	// Check the low bit to see if this is a reference
	if (ref & 1) == 0 {
		index := int(ref >> 1)
		if index >= len(cxt.stringTable) {
			cxt.saveError(os.NewError(fmt.Sprintf("Invalid string index: %d", index)))
			return ""
		}

		return cxt.stringTable[index]
	}

	length := int(ref >> 1)

	if length == 0 {
		return ""
	}

	str := cxt.ReadStringKnownLength(length)
	cxt.stringTable = append(cxt.stringTable, str)

	return str
}

func WriteStringAmf3(stream Writer, s string) os.Error {
	length := len(s)

	// TODO: Support outgoing string references.

	WriteUint29(stream, uint32((length<<1)+1))

	stream.Write([]byte(s))

	return nil
}

func (cxt *Decoder) readObjectAmf3() interface{} {

	ref := cxt.ReadUint29()

	if cxt.errored() {
		return nil
	}

	// Check the low bit to see if this is a reference
	if (ref & 1) == 0 {
		index := int(ref >> 1)
        if index >= len(cxt.objectTable) {
            cxt.saveError(os.NewError(fmt.Sprintf("Invalid object index: %d", index)))
            return nil
        }
		return cxt.objectTable[index]
	}

	class := cxt.readClassDefinitionAmf3(ref)

	object := AvmObject{}
	object.class = class
	object.dynamicFields = make(map[string]interface{})

	// Store the object in the table before doing any decoding.
	cxt.storeObjectInTable(&object)

	// Read static fields
	object.staticFields = make([]interface{}, len(class.properties))
	for i := range class.properties {
		value := cxt.ReadValueAmf3()
		object.staticFields[i] = value
	}

	if class.dynamic {
		// Parse dynamic fields
		for {
			name := cxt.readStringAmf3()
			if name == "" {
				break
			}

			value := cxt.ReadValueAmf3()
			object.dynamicFields[name] = value
		}
	}

	return &object
}

func writeObjectAmf3(stream Writer, value interface{}) os.Error {

	fmt.Printf("writeObjectAmf3 attempting to write a value of type %s\n",
		reflect.ValueOf(value).Type().Name())

	return nil
}

func writeAvmObject3(stream Writer, value *AvmObject) os.Error {
	// TODO: Support outgoing object references.

	// writeClassDefinitionAmf3 will also write the ref section.
	writeClassDefinitionAmf3(stream, value.class)

	return nil
}

func writeReflectedStructAmf3(stream Writer, value reflect.Value) os.Error {

    if (value.Kind() != reflect.Struct) {
        return os.NewError("writeReflectedStructAmf3 called with non-struct value")
    }

    // Ref is, non-object-ref, non-class-ref, non-externalizable, non-dynamic
    // TODO: Support object refs and class refs.
    ref := 0x2

    numFields := value.Type().NumField()

    ref += numFields << 4

    WriteUint29(stream, uint32(ref))

    // Class name
    WriteStringAmf3(stream, value.Type().Name())
    fmt.Printf("wrote class name = %s\n", value.Type().Name())

    // Property names
    for i := 0; i < numFields; i++ {
        structField := value.Type().Field(i)
        WriteStringAmf3(stream, structField.Name)
        fmt.Printf("wrote field name = %s\n", structField.Name)
    }

    // Property values
    for i := 0; i < numFields; i++ {
        writeReflectedValueAmf3(stream, value.Field(i))
    }

    return nil
}

func (cxt *Decoder) readClassDefinitionAmf3(ref uint32) *AvmClass {
	// Check for a reference to an existing class definition
	if (ref & 2) == 0 {
		return cxt.classTable[int(ref>>2)]
	}

	// Parse a class definition
	className := cxt.readStringAmf3()

	externalizable := ref&4 != 0
	dynamic := ref&8 != 0
	propertyCount := ref >> 4

	class := AvmClass{className, externalizable, dynamic, make([]string, propertyCount)}

	// Property names
	for i := uint32(0); i < propertyCount; i++ {
		class.properties[i] = cxt.readStringAmf3()
	}

	// Save the new class in the loopup table
	cxt.classTable = append(cxt.classTable, &class)

	return &class
}

func writeClassDefinitionAmf3(stream Writer, class *AvmClass) {
	// TODO: Support class references
	ref := uint32(0x2)

	if class.externalizable {
		ref += 0x4
	}
	if class.dynamic {
		ref += 0x8
	}

	ref += uint32(len(class.properties) << 4)

	WriteUint29(stream, ref)

	WriteStringAmf3(stream, class.name)

	// Property names
	for _, name := range class.properties {
		WriteStringAmf3(stream, name)
	}
}

func (cxt *Decoder) readArrayAmf3() interface{} {
	ref := cxt.ReadUint29()

	if cxt.errored() {
		return nil
	}

	// Check the low bit to see if this is a reference
	if (ref & 1) == 0 {
		index := int(ref >> 1)
		if index >= len(cxt.objectTable) {
			cxt.saveError(os.NewError(fmt.Sprintf("Invalid array reference: %d", index)))
			return nil
		}

		return cxt.objectTable[index]
	}

	elementCount := int(ref >> 1)

	// Read name-value pairs, if any.
	key := cxt.readStringAmf3()

	// No name-value pairs, return a flat Go array.
	if key == "" {
		result := make([]interface{}, elementCount)
		for i := 0; i < elementCount; i++ {
			result[i] = cxt.ReadValueAmf3()
		}
		return result
	}

	result := &AvmArray{}
	result.fields = make(map[string]interface{})

	// Store the object in the table before doing any decoding.
	cxt.storeObjectInTable(result)

	for key != "" {
		result.fields[key] = cxt.ReadValueAmf3()
		key = cxt.readStringAmf3()
	}

	// Read dense elements
	result.elements = make([]interface{}, elementCount)
	for i := 0; i < elementCount; i++ {
		result.elements[i] = cxt.ReadValueAmf3()
	}

	return result
}

func writeReflectedArrayAmf3(stream Writer, value reflect.Value) os.Error {

	elementCount := value.Len()

	// TODO: Support outgoing array references
	ref := (elementCount << 1) + 1

	WriteUint29(stream, uint32(ref))

	// Write an empty key since this is just a flat array.
	WriteStringAmf3(stream, "")

	for i := 0; i < elementCount; i++ {
		WriteValueAmf3(stream, value.Index(i).Interface())
	}
	return nil
}

func writeFlatArrayAmf3(stream Writer, value []interface{}) os.Error {
	elementCount := len(value)

	// TODO: Support outgoing array references
	ref := (elementCount << 1) + 1

	WriteUint29(stream, uint32(ref))

	// Write an empty key since this is just a flat array.
	WriteStringAmf3(stream, "")

	// Write dense elements
	for i := 0; i < elementCount; i++ {
		WriteValueAmf3(stream, value[i])
	}
	return nil
}

func writeMixedArray3(stream Writer, value *AvmArray) os.Error {
	elementCount := len(value.elements)

	// TODO: Support outgoing array references
	ref := (elementCount << 1) + 1

	WriteUint29(stream, uint32(ref))

	// Write fields
	for k, v := range value.fields {
		WriteStringAmf3(stream, k)
		WriteValueAmf3(stream, v)
	}

	// Write a null name to indicate the end of fields.
	WriteStringAmf3(stream, "")

	// Write dense elements
	for i := 0; i < elementCount; i++ {
		WriteValueAmf3(stream, value.elements[i])
	}
	return nil
}

func (cxt *Decoder) ReadValue() interface{} {
	if cxt.AmfVersion == 0 {
		return cxt.readValueAmf0()
	}

	return cxt.ReadValueAmf3()
}

func (cxt *Decoder) readValueAmf0() interface{} {

	typeMarker := cxt.ReadByte()

	if cxt.errored() {
		return nil
	}

	// Most AMF0 types are not yet supported.

	// Type markers
	switch typeMarker {
	case amf0_numberType:
		return cxt.ReadFloat64()
	case amf0_booleanType:
		val := cxt.ReadUint8()
		return val != 0
	case amf0_stringType:
		return cxt.ReadString()
	case amf0_objectType:
		result := map[string]interface{}{}
		for true {
			c1 := cxt.ReadByte()
			c2 := cxt.ReadByte()
			length := int(c1)<<8 + int(c2)
			name := cxt.ReadStringKnownLength(length)
			result[name] = cxt.readValueAmf0()
		}
		return result

	case amf0_movieClipType:
		fmt.Printf("Movie clip type not supported")
	case amf0_nullType:
		return nil
	case amf0_undefinedType:
		return nil
	case amf0_referenceType:
	case amf0_ecmaArrayType:
	case amf0_objectEndType:
	case amf0_strictArrayType:
	case amf0_dateType:
	case amf0_longStringType:
	case amf0_unsupporedType:
	case amf0_recordsetType:
	case amf0_xmlObjectType:
	case amf0_typedObjectType:
	case amf0_avmPlusObjectType:
		return cxt.ReadValueAmf3()
	}

	fmt.Printf("AMF0 type marker was not supported: %d", typeMarker)
	return nil
}

func (cxt *Decoder) ReadValueAmf3() interface{} {

	// Read type marker
	typeMarker := cxt.ReadByte()

	if cxt.errored() {
		return nil
	}

	// Flash Player 9 will sometimes wrap data as an AMF0 value, which just means that
	// there might be an additional type code (amf0_avmPlusObjectType), which we can
	// unambiguously ignore here.

	if typeMarker == amf0_avmPlusObjectType {
		typeMarker = cxt.ReadByte()
		if cxt.errored() {
			return nil
		}
	}

	switch typeMarker {
	case amf3_nullType, amf3_undefinedType:
		return nil
	case amf3_falseType:
		return false
	case amf3_trueType:
		return true
	case amf3_integerType:
		return cxt.ReadUint29()
	case amf3_doubleType:
		return cxt.ReadFloat64()
	case amf3_stringType:
		return cxt.readStringAmf3()
	case amf3_xmlType:
		// TODO
	case amf3_dateType:
		// TODO
	case amf3_objectType:
		return cxt.readObjectAmf3()
	case amf3_avmPlusXmlType:
		// TODO
	case amf3_byteArrayType:
		// TODO
	case amf3_arrayType:
		return cxt.readArrayAmf3()
	}

	cxt.saveError(os.NewError("AMF3 type marker was not supported"))
	return nil
}

func writeValueAmf3(stream Writer, value interface{}) os.Error {

    if value == nil {
		return writeByte(stream, amf3_nullType)
    }

    return writeReflectedValueAmf3(stream, reflect.ValueOf(value))

/*
	case *AvmArray:
		writeByte(stream, amf3_arrayType)
		arr, _ := value.(*AvmArray)
		writeByte(stream, amf3_arrayType)
		return writeMixedArray3(stream, arr)
	case *AvmObject:
		writeByte(stream, amf3_objectType)
		obj, _ := value.(*AvmObject)
		writeAvmObject3(stream, obj)
	default:
		fmt.Printf("writeValueAmf3 didn't recognize type: %s\n", reflect.ValueOf(value).Type().Name())
	}

	return nil
*/
}

func writeReflectedValueAmf3(stream Writer, value reflect.Value) os.Error {

	switch value.Kind() {
    case reflect.String:
		writeByte(stream, amf3_stringType)
		str, _ := value.Interface().(string)
		return WriteStringAmf3(stream, str)
    case reflect.Bool:
		if value.Bool() == false {
			return writeByte(stream, amf3_falseType)
		} else {
			return writeByte(stream, amf3_trueType)
		}
    case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		writeByte(stream, amf3_integerType)
		return WriteUint29(stream, uint32(value.Int()))
    case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		writeByte(stream, amf3_integerType)
		return WriteUint29(stream, uint32(value.Uint()))
    case reflect.Float32, reflect.Float64:
		writeByte(stream, amf3_doubleType)
        return WriteFloat64(stream, value.Float())
	case reflect.Array, reflect.Slice:
		writeByte(stream, amf3_arrayType)
		return writeReflectedArrayAmf3(stream, value)
	}

    return os.NewError(fmt.Sprintf("writeReflectedArrayAmf3 doesn't support kind: %v",
        value.Kind().String()))
}
