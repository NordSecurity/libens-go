
package ens

// #include <ens.h>
import "C"

import (
	"bytes"
	"fmt"
	"io"
	"unsafe"
	"encoding/binary"
	"errors"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
)



// This is needed, because as of go 1.24
// type RustBuffer C.RustBuffer cannot have methods,
// RustBuffer is treated as non-local type
type GoRustBuffer struct {
	inner C.RustBuffer
}

type RustBufferI interface {
	AsReader() *bytes.Reader
	Free()
	ToGoBytes() []byte
	Data() unsafe.Pointer
	Len() uint64
	Capacity() uint64
}

func RustBufferFromExternal(b RustBufferI) GoRustBuffer {
	return GoRustBuffer {
		inner: C.RustBuffer {
			capacity: C.uint64_t(b.Capacity()),
			len: C.uint64_t(b.Len()),
			data: (*C.uchar)(b.Data()),
		},
	}
}

func (cb GoRustBuffer) Capacity() uint64 {
	return uint64(cb.inner.capacity)
}

func (cb GoRustBuffer) Len() uint64 {
	return uint64(cb.inner.len)
}

func (cb GoRustBuffer) Data() unsafe.Pointer {
	return unsafe.Pointer(cb.inner.data)
}

func (cb GoRustBuffer) AsReader() *bytes.Reader {
	b := unsafe.Slice((*byte)(cb.inner.data), C.uint64_t(cb.inner.len))
	return bytes.NewReader(b)
}

func (cb GoRustBuffer) Free() {
	rustCall(func( status *C.RustCallStatus) bool {
		C.ffi_ens_rustbuffer_free(cb.inner, status)
		return false
	})
}

func (cb GoRustBuffer) ToGoBytes() []byte {
	return C.GoBytes(unsafe.Pointer(cb.inner.data), C.int(cb.inner.len))
}


func stringToRustBuffer(str string) C.RustBuffer {
	return bytesToRustBuffer([]byte(str))
}

func bytesToRustBuffer(b []byte) C.RustBuffer {
	if len(b) == 0 {
		return C.RustBuffer{}
	}
	// We can pass the pointer along here, as it is pinned
	// for the duration of this call
	foreign := C.ForeignBytes {
		len: C.int(len(b)),
		data: (*C.uchar)(unsafe.Pointer(&b[0])),
	}
	
	return rustCall(func( status *C.RustCallStatus) C.RustBuffer {
		return C.ffi_ens_rustbuffer_from_bytes(foreign, status)
	})
}


type BufLifter[GoType any] interface {
	Lift(value RustBufferI) GoType
}

type BufLowerer[GoType any] interface {
	Lower(value GoType) C.RustBuffer
}

type BufReader[GoType any] interface {
	Read(reader io.Reader) GoType
}

type BufWriter[GoType any] interface {
	Write(writer io.Writer, value GoType)
}

func LowerIntoRustBuffer[GoType any](bufWriter BufWriter[GoType], value GoType) C.RustBuffer {
	// This might be not the most efficient way but it does not require knowing allocation size
	// beforehand
	var buffer bytes.Buffer
	bufWriter.Write(&buffer, value)

	bytes, err := io.ReadAll(&buffer)
	if err != nil {
		panic(fmt.Errorf("reading written data: %w", err))
	}
	return bytesToRustBuffer(bytes)
}

func LiftFromRustBuffer[GoType any](bufReader BufReader[GoType], rbuf RustBufferI) GoType {
	defer rbuf.Free()
	reader := rbuf.AsReader()
	item := bufReader.Read(reader)
	if reader.Len() > 0 {
		// TODO: Remove this
		leftover, _ := io.ReadAll(reader)
		panic(fmt.Errorf("Junk remaining in buffer after lifting: %s", string(leftover)))
	}
	return item
}



func rustCallWithError[E any, U any](converter BufReader[*E], callback func(*C.RustCallStatus) U) (U, *E) {
	var status C.RustCallStatus
	returnValue := callback(&status)
	err := checkCallStatus(converter, status)
	return returnValue, err
}

func checkCallStatus[E any](converter BufReader[*E], status C.RustCallStatus) *E {
	switch status.code {
	case 0:
		return nil
	case 1:
		return LiftFromRustBuffer(converter, GoRustBuffer { inner: status.errorBuf })
	case 2:
		// when the rust code sees a panic, it tries to construct a rustBuffer
		// with the message.  but if that code panics, then it just sends back
		// an empty buffer.
		if status.errorBuf.len > 0 {
			panic(fmt.Errorf("%s", FfiConverterStringINSTANCE.Lift(GoRustBuffer { inner: status.errorBuf })))
		} else {
			panic(fmt.Errorf("Rust panicked while handling Rust panic"))
		}
	default:
		panic(fmt.Errorf("unknown status code: %d", status.code))
	}
}

func checkCallStatusUnknown(status C.RustCallStatus) error {
	switch status.code {
	case 0:
		return nil
	case 1:
		panic(fmt.Errorf("function not returning an error returned an error"))
	case 2:
		// when the rust code sees a panic, it tries to construct a C.RustBuffer
		// with the message.  but if that code panics, then it just sends back
		// an empty buffer.
		if status.errorBuf.len > 0 {
			panic(fmt.Errorf("%s", FfiConverterStringINSTANCE.Lift(GoRustBuffer {
				inner: status.errorBuf,
			})))
		} else {
			panic(fmt.Errorf("Rust panicked while handling Rust panic"))
		}
	default:
		return fmt.Errorf("unknown status code: %d", status.code)
	}
}

func rustCall[U any](callback func(*C.RustCallStatus) U) U {
	returnValue, err := rustCallWithError[error](nil, callback)
	if err != nil {
		panic(err)
	}
	return returnValue
}

type NativeError interface {
	AsError() error
}


func writeInt8(writer io.Writer, value int8) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint8(writer io.Writer, value uint8) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt16(writer io.Writer, value int16) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint16(writer io.Writer, value uint16) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt32(writer io.Writer, value int32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint32(writer io.Writer, value uint32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt64(writer io.Writer, value int64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint64(writer io.Writer, value uint64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeFloat32(writer io.Writer, value float32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeFloat64(writer io.Writer, value float64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}


func readInt8(reader io.Reader) int8 {
	var result int8
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint8(reader io.Reader) uint8 {
	var result uint8
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt16(reader io.Reader) int16 {
	var result int16
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint16(reader io.Reader) uint16 {
	var result uint16
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt32(reader io.Reader) int32 {
	var result int32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint32(reader io.Reader) uint32 {
	var result uint32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt64(reader io.Reader) int64 {
	var result int64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint64(reader io.Reader) uint64 {
	var result uint64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readFloat32(reader io.Reader) float32 {
	var result float32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readFloat64(reader io.Reader) float64 {
	var result float64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func init() {
        
        FfiConverterCallbackInterfaceErrorNotificationCallbackINSTANCE.register();
        FfiConverterCallbackInterfaceLogCallbackINSTANCE.register();
        FfiConverterCallbackInterfaceProtectCallbackINSTANCE.register();
        uniffiCheckChecksums()
}


func uniffiCheckChecksums() {
	// Get the bindings contract version from our ComponentInterface
	bindingsContractVersion := 26
	// Get the scaffolding contract version by calling the into the dylib
	scaffoldingContractVersion := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.ffi_ens_uniffi_contract_version()
	})
	if bindingsContractVersion != int(scaffoldingContractVersion) {
		// If this happens try cleaning and rebuilding your project
		panic("ens: UniFFI contract version mismatch")
	}
	{
	checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
		return C.uniffi_ens_checksum_func_connect()
	})
	if checksum != 60288 {
		// If this happens try cleaning and rebuilding your project
		panic("ens: uniffi_ens_checksum_func_connect: UniFFI API checksum mismatch")
	}
	}
	{
	checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
		return C.uniffi_ens_checksum_func_deinit()
	})
	if checksum != 45885 {
		// If this happens try cleaning and rebuilding your project
		panic("ens: uniffi_ens_checksum_func_deinit: UniFFI API checksum mismatch")
	}
	}
	{
	checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
		return C.uniffi_ens_checksum_func_get_memory_usage()
	})
	if checksum != 23754 {
		// If this happens try cleaning and rebuilding your project
		panic("ens: uniffi_ens_checksum_func_get_memory_usage: UniFFI API checksum mismatch")
	}
	}
	{
	checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
		return C.uniffi_ens_checksum_func_get_version()
	})
	if checksum != 17699 {
		// If this happens try cleaning and rebuilding your project
		panic("ens: uniffi_ens_checksum_func_get_version: UniFFI API checksum mismatch")
	}
	}
	{
	checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
		return C.uniffi_ens_checksum_func_init()
	})
	if checksum != 3391 {
		// If this happens try cleaning and rebuilding your project
		panic("ens: uniffi_ens_checksum_func_init: UniFFI API checksum mismatch")
	}
	}
	{
	checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
		return C.uniffi_ens_checksum_func_set_log_callback()
	})
	if checksum != 20505 {
		// If this happens try cleaning and rebuilding your project
		panic("ens: uniffi_ens_checksum_func_set_log_callback: UniFFI API checksum mismatch")
	}
	}
	{
	checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
		return C.uniffi_ens_checksum_method_config_set_allow_only_pq()
	})
	if checksum != 35895 {
		// If this happens try cleaning and rebuilding your project
		panic("ens: uniffi_ens_checksum_method_config_set_allow_only_pq: UniFFI API checksum mismatch")
	}
	}
	{
	checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
		return C.uniffi_ens_checksum_method_config_set_backoff_initial()
	})
	if checksum != 41711 {
		// If this happens try cleaning and rebuilding your project
		panic("ens: uniffi_ens_checksum_method_config_set_backoff_initial: UniFFI API checksum mismatch")
	}
	}
	{
	checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
		return C.uniffi_ens_checksum_method_config_set_backoff_maximal()
	})
	if checksum != 52425 {
		// If this happens try cleaning and rebuilding your project
		panic("ens: uniffi_ens_checksum_method_config_set_backoff_maximal: UniFFI API checksum mismatch")
	}
	}
	{
	checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
		return C.uniffi_ens_checksum_method_config_set_buffer_size()
	})
	if checksum != 22653 {
		// If this happens try cleaning and rebuilding your project
		panic("ens: uniffi_ens_checksum_method_config_set_buffer_size: UniFFI API checksum mismatch")
	}
	}
	{
	checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
		return C.uniffi_ens_checksum_method_config_set_keepalive_interval()
	})
	if checksum != 28617 {
		// If this happens try cleaning and rebuilding your project
		panic("ens: uniffi_ens_checksum_method_config_set_keepalive_interval: UniFFI API checksum mismatch")
	}
	}
	{
	checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
		return C.uniffi_ens_checksum_method_config_set_keepalive_timeout()
	})
	if checksum != 47608 {
		// If this happens try cleaning and rebuilding your project
		panic("ens: uniffi_ens_checksum_method_config_set_keepalive_timeout: UniFFI API checksum mismatch")
	}
	}
	{
	checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
		return C.uniffi_ens_checksum_method_config_set_root_certificate_override()
	})
	if checksum != 440 {
		// If this happens try cleaning and rebuilding your project
		panic("ens: uniffi_ens_checksum_method_config_set_root_certificate_override: UniFFI API checksum mismatch")
	}
	}
	{
	checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
		return C.uniffi_ens_checksum_method_connection_shutdown()
	})
	if checksum != 16517 {
		// If this happens try cleaning and rebuilding your project
		panic("ens: uniffi_ens_checksum_method_connection_shutdown: UniFFI API checksum mismatch")
	}
	}
	{
	checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
		return C.uniffi_ens_checksum_constructor_config_new()
	})
	if checksum != 47748 {
		// If this happens try cleaning and rebuilding your project
		panic("ens: uniffi_ens_checksum_constructor_config_new: UniFFI API checksum mismatch")
	}
	}
	{
	checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
		return C.uniffi_ens_checksum_method_errornotificationcallback_notify()
	})
	if checksum != 12492 {
		// If this happens try cleaning and rebuilding your project
		panic("ens: uniffi_ens_checksum_method_errornotificationcallback_notify: UniFFI API checksum mismatch")
	}
	}
	{
	checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
		return C.uniffi_ens_checksum_method_errornotificationcallback_disconnected()
	})
	if checksum != 59703 {
		// If this happens try cleaning and rebuilding your project
		panic("ens: uniffi_ens_checksum_method_errornotificationcallback_disconnected: UniFFI API checksum mismatch")
	}
	}
	{
	checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
		return C.uniffi_ens_checksum_method_logcallback_log()
	})
	if checksum != 2173 {
		// If this happens try cleaning and rebuilding your project
		panic("ens: uniffi_ens_checksum_method_logcallback_log: UniFFI API checksum mismatch")
	}
	}
	{
	checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
		return C.uniffi_ens_checksum_method_protectcallback_protect()
	})
	if checksum != 24331 {
		// If this happens try cleaning and rebuilding your project
		panic("ens: uniffi_ens_checksum_method_protectcallback_protect: UniFFI API checksum mismatch")
	}
	}
}




type FfiConverterUint32 struct{}

var FfiConverterUint32INSTANCE = FfiConverterUint32{}

func (FfiConverterUint32) Lower(value uint32) C.uint32_t {
	return C.uint32_t(value)
}

func (FfiConverterUint32) Write(writer io.Writer, value uint32) {
	writeUint32(writer, value)
}

func (FfiConverterUint32) Lift(value C.uint32_t) uint32 {
	return uint32(value)
}

func (FfiConverterUint32) Read(reader io.Reader) uint32 {
	return readUint32(reader)
}

type FfiDestroyerUint32 struct {}

func (FfiDestroyerUint32) Destroy(_ uint32) {}


type FfiConverterInt32 struct{}

var FfiConverterInt32INSTANCE = FfiConverterInt32{}

func (FfiConverterInt32) Lower(value int32) C.int32_t {
	return C.int32_t(value)
}

func (FfiConverterInt32) Write(writer io.Writer, value int32) {
	writeInt32(writer, value)
}

func (FfiConverterInt32) Lift(value C.int32_t) int32 {
	return int32(value)
}

func (FfiConverterInt32) Read(reader io.Reader) int32 {
	return readInt32(reader)
}

type FfiDestroyerInt32 struct {}

func (FfiDestroyerInt32) Destroy(_ int32) {}


type FfiConverterUint64 struct{}

var FfiConverterUint64INSTANCE = FfiConverterUint64{}

func (FfiConverterUint64) Lower(value uint64) C.uint64_t {
	return C.uint64_t(value)
}

func (FfiConverterUint64) Write(writer io.Writer, value uint64) {
	writeUint64(writer, value)
}

func (FfiConverterUint64) Lift(value C.uint64_t) uint64 {
	return uint64(value)
}

func (FfiConverterUint64) Read(reader io.Reader) uint64 {
	return readUint64(reader)
}

type FfiDestroyerUint64 struct {}

func (FfiDestroyerUint64) Destroy(_ uint64) {}


type FfiConverterBool struct{}

var FfiConverterBoolINSTANCE = FfiConverterBool{}

func (FfiConverterBool) Lower(value bool) C.int8_t {
	if value {
		return C.int8_t(1)
	}
	return C.int8_t(0)
}

func (FfiConverterBool) Write(writer io.Writer, value bool) {
	if value {
		writeInt8(writer, 1)
	} else {
		writeInt8(writer, 0)
	}
}

func (FfiConverterBool) Lift(value C.int8_t) bool {
	return value != 0
}

func (FfiConverterBool) Read(reader io.Reader) bool {
	return readInt8(reader) != 0
}

type FfiDestroyerBool struct {}

func (FfiDestroyerBool) Destroy(_ bool) {}


type FfiConverterString struct{}

var FfiConverterStringINSTANCE = FfiConverterString{}

func (FfiConverterString) Lift(rb RustBufferI) string {
	defer rb.Free()
	reader := rb.AsReader()
	b, err := io.ReadAll(reader)
	if err != nil {
		panic(fmt.Errorf("reading reader: %w", err))
	}
	return string(b)
}

func (FfiConverterString) Read(reader io.Reader) string {
	length := readInt32(reader)
	buffer := make([]byte, length)
	read_length, err := reader.Read(buffer)
	if err != nil && err != io.EOF {
		panic(err)
	}
	if read_length != int(length) {
		panic(fmt.Errorf("bad read length when reading string, expected %d, read %d", length, read_length))
	}
	return string(buffer)
}

func (FfiConverterString) Lower(value string) C.RustBuffer {
	return stringToRustBuffer(value)
}

func (FfiConverterString) Write(writer io.Writer, value string) {
	if len(value) > math.MaxInt32 {
		panic("String is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	write_length, err := io.WriteString(writer, value)
	if err != nil {
		panic(err)
	}
	if write_length != len(value) {
		panic(fmt.Errorf("bad write length when writing string, expected %d, written %d", len(value), write_length))
	}
}

type FfiDestroyerString struct {}

func (FfiDestroyerString) Destroy(_ string) {}


type FfiConverterBytes struct{}

var FfiConverterBytesINSTANCE = FfiConverterBytes{}

func (c FfiConverterBytes) Lower(value []byte) C.RustBuffer {
	return LowerIntoRustBuffer[[]byte](c, value)
}

func (c FfiConverterBytes) Write(writer io.Writer, value []byte) {
	if len(value) > math.MaxInt32 {
		panic("[]byte is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	write_length, err := writer.Write(value)
	if err != nil {
		panic(err)
	}
	if write_length != len(value) {
		panic(fmt.Errorf("bad write length when writing []byte, expected %d, written %d", len(value), write_length))
	}
}

func (c FfiConverterBytes) Lift(rb RustBufferI) []byte {
	return LiftFromRustBuffer[[]byte](c, rb)
}

func (c FfiConverterBytes) Read(reader io.Reader) []byte {
	length := readInt32(reader)
	buffer := make([]byte, length)
	read_length, err := reader.Read(buffer)
	if err != nil && err != io.EOF {
		panic(err)
	}
	if read_length != int(length) {
		panic(fmt.Errorf("bad read length when reading []byte, expected %d, read %d", length, read_length))
	}
	return buffer
}

type FfiDestroyerBytes struct {}

func (FfiDestroyerBytes) Destroy(_ []byte) {}




// Below is an implementation of synchronization requirements outlined in the link.
// https://github.com/mozilla/uniffi-rs/blob/0dc031132d9493ca812c3af6e7dd60ad2ea95bf0/uniffi_bindgen/src/bindings/kotlin/templates/ObjectRuntime.kt#L31

type FfiObject struct {
	pointer unsafe.Pointer
	callCounter atomic.Int64
	cloneFunction func(unsafe.Pointer, *C.RustCallStatus) unsafe.Pointer
	freeFunction func(unsafe.Pointer, *C.RustCallStatus)
	destroyed atomic.Bool
}

func newFfiObject(
	pointer unsafe.Pointer, 
	cloneFunction func(unsafe.Pointer, *C.RustCallStatus) unsafe.Pointer, 
	freeFunction func(unsafe.Pointer, *C.RustCallStatus),
) FfiObject {
	return FfiObject {
		pointer: pointer,
		cloneFunction: cloneFunction, 
		freeFunction: freeFunction,
	}
}

func (ffiObject *FfiObject)incrementPointer(debugName string) unsafe.Pointer {
	for {
		counter := ffiObject.callCounter.Load()
		if counter <= -1 {
			panic(fmt.Errorf("%v object has already been destroyed", debugName))
		}
		if counter == math.MaxInt64 {
			panic(fmt.Errorf("%v object call counter would overflow", debugName))
		}
		if ffiObject.callCounter.CompareAndSwap(counter, counter + 1) {
			break
		}
	}

	return rustCall(func(status *C.RustCallStatus) unsafe.Pointer {
		return ffiObject.cloneFunction(ffiObject.pointer, status)
	})
}

func (ffiObject *FfiObject)decrementPointer() {
	if ffiObject.callCounter.Add(-1) == -1 {
		ffiObject.freeRustArcPtr()
	}
}

func (ffiObject *FfiObject)destroy() {
	if ffiObject.destroyed.CompareAndSwap(false, true) {
		if ffiObject.callCounter.Add(-1) == -1 {
			ffiObject.freeRustArcPtr()
		}
	}
}

func (ffiObject *FfiObject)freeRustArcPtr() {
	rustCall(func(status *C.RustCallStatus) int32 {
		ffiObject.freeFunction(ffiObject.pointer, status)
		return 0
	})
}
type ConfigInterface interface {
	// When enabled, the TLS handshake with the ENS server offers only the
	// post quantum X25519MLKEM768 key exchange.
	// [default true]
	SetAllowOnlyPq(allowOnlyPq bool) 
	// Delay before the first reconnection attempt, in seconds. It doubles
	// with every failed attempt, up to the maximal backoff. Must be non zero.
	// [default 2s]
	SetBackoffInitial(seconds uint32) 
	// Upper limit for the reconnection delay, in seconds. Must not be smaller
	// than the initial backoff.
	// When set to null, the delay keeps growing without a limit.
	// [default 120s]
	SetBackoffMaximal(seconds *uint32) 
	// Capacity of the internal queue holding error notifications before they
	// are passed to the `ErrorNotificationCallback`. Must be non zero.
	// [default 5]
	SetBufferSize(size uint32) 
	// Interval between the keep alive messages sent over the ENS connection, in
	// seconds. When the underlying tcp connection stops working, the dead
	// connection will be detected after at most keepalive interval + keepalive
	// timeout seconds and a new connection will be created after the ENS backoff
	// elapses.
	// When set to null, the keep alives are disabled. Setting it to 0 resets it
	// back to the default.
	// [default 120s]
	SetKeepaliveInterval(seconds *uint32) 
	// How long to wait for a response to a keep alive message before considering
	// the ENS connection dead, in seconds. Only used when the keepalive interval
	// is set.
	// When set to null, the underlying http client default is used. Setting it to
	// 0 resets it back to the default.
	// [default 20s]
	SetKeepaliveTimeout(seconds *uint32) 
	// DER encoded root certificate used to verify the ENS server instead of
	// the one built into the library. Intended for testing.
	// When set to null, the built in certificate is used.
	// [default null]
	SetRootCertificateOverride(override *[]byte) 
}
type Config struct {
	ffiObject FfiObject
}
func NewConfig() *Config {
	return FfiConverterConfigINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_ens_fn_constructor_config_new(_uniffiStatus)
	}))
}




// When enabled, the TLS handshake with the ENS server offers only the
// post quantum X25519MLKEM768 key exchange.
// [default true]
func (_self *Config) SetAllowOnlyPq(allowOnlyPq bool)  {
	_pointer := _self.ffiObject.incrementPointer("*Config")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_ens_fn_method_config_set_allow_only_pq(
		_pointer,FfiConverterBoolINSTANCE.Lower(allowOnlyPq),_uniffiStatus)
		return false
	})
}

// Delay before the first reconnection attempt, in seconds. It doubles
// with every failed attempt, up to the maximal backoff. Must be non zero.
// [default 2s]
func (_self *Config) SetBackoffInitial(seconds uint32)  {
	_pointer := _self.ffiObject.incrementPointer("*Config")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_ens_fn_method_config_set_backoff_initial(
		_pointer,FfiConverterUint32INSTANCE.Lower(seconds),_uniffiStatus)
		return false
	})
}

// Upper limit for the reconnection delay, in seconds. Must not be smaller
// than the initial backoff.
// When set to null, the delay keeps growing without a limit.
// [default 120s]
func (_self *Config) SetBackoffMaximal(seconds *uint32)  {
	_pointer := _self.ffiObject.incrementPointer("*Config")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_ens_fn_method_config_set_backoff_maximal(
		_pointer,FfiConverterOptionalUint32INSTANCE.Lower(seconds),_uniffiStatus)
		return false
	})
}

// Capacity of the internal queue holding error notifications before they
// are passed to the `ErrorNotificationCallback`. Must be non zero.
// [default 5]
func (_self *Config) SetBufferSize(size uint32)  {
	_pointer := _self.ffiObject.incrementPointer("*Config")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_ens_fn_method_config_set_buffer_size(
		_pointer,FfiConverterUint32INSTANCE.Lower(size),_uniffiStatus)
		return false
	})
}

// Interval between the keep alive messages sent over the ENS connection, in
// seconds. When the underlying tcp connection stops working, the dead
// connection will be detected after at most keepalive interval + keepalive
// timeout seconds and a new connection will be created after the ENS backoff
// elapses.
// When set to null, the keep alives are disabled. Setting it to 0 resets it
// back to the default.
// [default 120s]
func (_self *Config) SetKeepaliveInterval(seconds *uint32)  {
	_pointer := _self.ffiObject.incrementPointer("*Config")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_ens_fn_method_config_set_keepalive_interval(
		_pointer,FfiConverterOptionalUint32INSTANCE.Lower(seconds),_uniffiStatus)
		return false
	})
}

// How long to wait for a response to a keep alive message before considering
// the ENS connection dead, in seconds. Only used when the keepalive interval
// is set.
// When set to null, the underlying http client default is used. Setting it to
// 0 resets it back to the default.
// [default 20s]
func (_self *Config) SetKeepaliveTimeout(seconds *uint32)  {
	_pointer := _self.ffiObject.incrementPointer("*Config")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_ens_fn_method_config_set_keepalive_timeout(
		_pointer,FfiConverterOptionalUint32INSTANCE.Lower(seconds),_uniffiStatus)
		return false
	})
}

// DER encoded root certificate used to verify the ENS server instead of
// the one built into the library. Intended for testing.
// When set to null, the built in certificate is used.
// [default null]
func (_self *Config) SetRootCertificateOverride(override *[]byte)  {
	_pointer := _self.ffiObject.incrementPointer("*Config")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_ens_fn_method_config_set_root_certificate_override(
		_pointer,FfiConverterOptionalBytesINSTANCE.Lower(override),_uniffiStatus)
		return false
	})
}
func (object *Config) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterConfig struct {}

var FfiConverterConfigINSTANCE = FfiConverterConfig{}


func (c FfiConverterConfig) Lift(pointer unsafe.Pointer) *Config {
	result := &Config {
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_ens_fn_clone_config(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_ens_fn_free_config(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Config).Destroy)
	return result
}

func (c FfiConverterConfig) Read(reader io.Reader) *Config {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterConfig) Lower(value *Config) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Config")
	defer value.ffiObject.decrementPointer()
	return pointer
	
}

func (c FfiConverterConfig) Write(writer io.Writer, value *Config) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerConfig struct {}

func (_ FfiDestroyerConfig) Destroy(value *Config) {
		value.Destroy()
}





// Handle to a live ENS session created by `connect()`.
//
// Holding the returned `Connection` keeps the session and its
// background task alive. Dropping it without calling `shutdown()` is
// allowed and will terminate the session, but `shutdown()` is
// preferred because it lets the caller observe termination failures.
type ConnectionInterface interface {
	// Terminates the ENS session and waits for the background task to
	// stop. After this returns, no further callbacks will fire on the
	// `ErrorNotificationCallback` that was passed to `connect()`
	// (except one final `disconnected("shutdown")` call).
	//
	// Safe to call multiple times; subsequent calls are no-ops.
	Shutdown() error
}
// Handle to a live ENS session created by `connect()`.
//
// Holding the returned `Connection` keeps the session and its
// background task alive. Dropping it without calling `shutdown()` is
// allowed and will terminate the session, but `shutdown()` is
// preferred because it lets the caller observe termination failures.
type Connection struct {
	ffiObject FfiObject
}




// Terminates the ENS session and waits for the background task to
// stop. After this returns, no further callbacks will fire on the
// `ErrorNotificationCallback` that was passed to `connect()`
// (except one final `disconnected("shutdown")` call).
//
// Safe to call multiple times; subsequent calls are no-ops.
func (_self *Connection) Shutdown() error {
	_pointer := _self.ffiObject.incrementPointer("*Connection")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[EnsError](FfiConverterEnsError{},func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_ens_fn_method_connection_shutdown(
		_pointer,_uniffiStatus)
		return false
	})
		return _uniffiErr.AsError()
}
func (object *Connection) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterConnection struct {}

var FfiConverterConnectionINSTANCE = FfiConverterConnection{}


func (c FfiConverterConnection) Lift(pointer unsafe.Pointer) *Connection {
	result := &Connection {
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_ens_fn_clone_connection(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_ens_fn_free_connection(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Connection).Destroy)
	return result
}

func (c FfiConverterConnection) Read(reader io.Reader) *Connection {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterConnection) Lower(value *Connection) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Connection")
	defer value.ffiObject.decrementPointer()
	return pointer
	
}

func (c FfiConverterConnection) Write(writer io.Writer, value *Connection) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerConnection struct {}

func (_ FfiDestroyerConnection) Destroy(value *Connection) {
		value.Destroy()
}





// A single connection-error notification pushed from the VPN server.
type ConnectionErrorNotification struct {
	// Machine-readable classification of the notification.
	Kind ConnectionErrorNotificationKind
	// Free-form, server-supplied context. May be empty. Not
	// machine-parseable - intended for human display or logging only.
	AdditionalInfo *string
}

func (r *ConnectionErrorNotification) Destroy() {
		FfiDestroyerConnectionErrorNotificationKind{}.Destroy(r.Kind);
		FfiDestroyerOptionalString{}.Destroy(r.AdditionalInfo);
}

type FfiConverterConnectionErrorNotification struct {}

var FfiConverterConnectionErrorNotificationINSTANCE = FfiConverterConnectionErrorNotification{}

func (c FfiConverterConnectionErrorNotification) Lift(rb RustBufferI) ConnectionErrorNotification {
	return LiftFromRustBuffer[ConnectionErrorNotification](c, rb)
}

func (c FfiConverterConnectionErrorNotification) Read(reader io.Reader) ConnectionErrorNotification {
	return ConnectionErrorNotification {
			FfiConverterConnectionErrorNotificationKindINSTANCE.Read(reader),
			FfiConverterOptionalStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterConnectionErrorNotification) Lower(value ConnectionErrorNotification) C.RustBuffer {
	return LowerIntoRustBuffer[ConnectionErrorNotification](c, value)
}

func (c FfiConverterConnectionErrorNotification) Write(writer io.Writer, value ConnectionErrorNotification) {
		FfiConverterConnectionErrorNotificationKindINSTANCE.Write(writer, value.Kind);
		FfiConverterOptionalStringINSTANCE.Write(writer, value.AdditionalInfo);
}

type FfiDestroyerConnectionErrorNotification struct {}

func (_ FfiDestroyerConnectionErrorNotification) Destroy(value ConnectionErrorNotification) {
	value.Destroy()
}


// Service credentials used to authenticate to ENS for OpenVPN and
// NordWhisper VPN sessions.
//
// These are the same `{username, password}` the client already sends to
// the VPN server itself (OpenVPN's management-socket auth,
// NordWhisper's `X-Auth-Password` header), obtained from the NordVPN
// backend via `/v1/users/services/credentials`. They are backend-issued
// high-entropy random strings, not user-chosen passwords.
type Credentials struct {
	// Service username (`Credentials.OpenVPNUsername` on the client
	// side).
	Username HiddenString
	// Service password (`Credentials.OpenVPNPassword` on the client
	// side). Treat as secret; do not log.
	Password HiddenString
	// Which VPN protocol this credential pair authenticates.
	Kind CredentialsKind
}

func (r *Credentials) Destroy() {
		FfiDestroyerTypeHiddenString{}.Destroy(r.Username);
		FfiDestroyerTypeHiddenString{}.Destroy(r.Password);
		FfiDestroyerCredentialsKind{}.Destroy(r.Kind);
}

type FfiConverterCredentials struct {}

var FfiConverterCredentialsINSTANCE = FfiConverterCredentials{}

func (c FfiConverterCredentials) Lift(rb RustBufferI) Credentials {
	return LiftFromRustBuffer[Credentials](c, rb)
}

func (c FfiConverterCredentials) Read(reader io.Reader) Credentials {
	return Credentials {
			FfiConverterTypeHiddenStringINSTANCE.Read(reader),
			FfiConverterTypeHiddenStringINSTANCE.Read(reader),
			FfiConverterCredentialsKindINSTANCE.Read(reader),
	}
}

func (c FfiConverterCredentials) Lower(value Credentials) C.RustBuffer {
	return LowerIntoRustBuffer[Credentials](c, value)
}

func (c FfiConverterCredentials) Write(writer io.Writer, value Credentials) {
		FfiConverterTypeHiddenStringINSTANCE.Write(writer, value.Username);
		FfiConverterTypeHiddenStringINSTANCE.Write(writer, value.Password);
		FfiConverterCredentialsKindINSTANCE.Write(writer, value.Kind);
}

type FfiDestroyerCredentials struct {}

func (_ FfiDestroyerCredentials) Destroy(value Credentials) {
	value.Destroy()
}


// Public/private key material used to authenticate to ENS for WireGuard
// (NordLynx) VPN sessions.
//
// ENS authentication for NordLynx uses an ECDH between the client's
// local NordLynx private key and the VPN server's public key, matching
// the identity model of the WireGuard tunnel itself. Both key halves
// are required.
type Keys struct {
	// Client's private key.
	LocalPrivateKey HiddenBytes
	// VPN server's public key - the same public key used to establish
	// the WireGuard tunnel.
	VpnPublicKey HiddenBytes
	// Which VPN protocol this key material authenticates.
	Kind KeyKind
}

func (r *Keys) Destroy() {
		FfiDestroyerTypeHiddenBytes{}.Destroy(r.LocalPrivateKey);
		FfiDestroyerTypeHiddenBytes{}.Destroy(r.VpnPublicKey);
		FfiDestroyerKeyKind{}.Destroy(r.Kind);
}

type FfiConverterKeys struct {}

var FfiConverterKeysINSTANCE = FfiConverterKeys{}

func (c FfiConverterKeys) Lift(rb RustBufferI) Keys {
	return LiftFromRustBuffer[Keys](c, rb)
}

func (c FfiConverterKeys) Read(reader io.Reader) Keys {
	return Keys {
			FfiConverterTypeHiddenBytesINSTANCE.Read(reader),
			FfiConverterTypeHiddenBytesINSTANCE.Read(reader),
			FfiConverterKeyKindINSTANCE.Read(reader),
	}
}

func (c FfiConverterKeys) Lower(value Keys) C.RustBuffer {
	return LowerIntoRustBuffer[Keys](c, value)
}

func (c FfiConverterKeys) Write(writer io.Writer, value Keys) {
		FfiConverterTypeHiddenBytesINSTANCE.Write(writer, value.LocalPrivateKey);
		FfiConverterTypeHiddenBytesINSTANCE.Write(writer, value.VpnPublicKey);
		FfiConverterKeyKindINSTANCE.Write(writer, value.Kind);
}

type FfiDestroyerKeys struct {}

func (_ FfiDestroyerKeys) Destroy(value Keys) {
	value.Destroy()
}



// Authentication material for an ENS session.
//
// Tagged union: exactly one variant is used per `connect()` call, and
// the variant must match the VPN protocol whose connection ENS is
// monitoring - `Credentials` for OpenVPN and NordWhisper, `Keys` for
// NordLynx (WireGuard).
type Authentication interface {
	Destroy()
}
// Username/password authentication for OpenVPN and NordWhisper.
type AuthenticationWithCredentials struct {
	Credentials Credentials
}

func (e AuthenticationWithCredentials) Destroy() {
		FfiDestroyerCredentials{}.Destroy(e.Credentials);
}
// Key-based authentication for NordLynx (WireGuard).
type AuthenticationWithKeys struct {
	Keys Keys
}

func (e AuthenticationWithKeys) Destroy() {
		FfiDestroyerKeys{}.Destroy(e.Keys);
}

type FfiConverterAuthentication struct {}

var FfiConverterAuthenticationINSTANCE = FfiConverterAuthentication{}

func (c FfiConverterAuthentication) Lift(rb RustBufferI) Authentication {
	return LiftFromRustBuffer[Authentication](c, rb)
}

func (c FfiConverterAuthentication) Lower(value Authentication) C.RustBuffer {
	return LowerIntoRustBuffer[Authentication](c, value)
}
func (FfiConverterAuthentication) Read(reader io.Reader) Authentication {
	id := readInt32(reader)
	switch (id) {
		case 1:
			return AuthenticationWithCredentials{
				FfiConverterCredentialsINSTANCE.Read(reader),
			};
		case 2:
			return AuthenticationWithKeys{
				FfiConverterKeysINSTANCE.Read(reader),
			};
		default:
			panic(fmt.Sprintf("invalid enum value %v in FfiConverterAuthentication.Read()", id));
	}
}

func (FfiConverterAuthentication) Write(writer io.Writer, value Authentication) {
	switch variant_value := value.(type) {
		case AuthenticationWithCredentials:
			writeInt32(writer, 1)
			FfiConverterCredentialsINSTANCE.Write(writer, variant_value.Credentials)
		case AuthenticationWithKeys:
			writeInt32(writer, 2)
			FfiConverterKeysINSTANCE.Write(writer, variant_value.Keys)
		default:
			_ = variant_value
			panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterAuthentication.Write", value))
	}
}

type FfiDestroyerAuthentication struct {}

func (_ FfiDestroyerAuthentication) Destroy(value Authentication) {
	value.Destroy()
}




// Classification of a connection-level error notification received from
// the VPN server. These describe problems with the VPN connection itself,
// not with the ENS client-to-server link.
type ConnectionErrorNotificationKind interface {
	Destroy()
}
// The server sent a notification kind this version of the library
// does not recognize. The actuall enum value is stored in the `kind`
// field.
type ConnectionErrorNotificationKindUnknown struct {
	Kind int32
}

func (e ConnectionErrorNotificationKindUnknown) Destroy() {
		FfiDestroyerInt32{}.Destroy(e.Kind);
}
// The account has reached its concurrent-device limit.
type ConnectionErrorNotificationKindConnectionLimitReached struct {
}

func (e ConnectionErrorNotificationKindConnectionLimitReached) Destroy() {
}
// The server is going into maintenance. This is the only notification
// kind for which the caller can automatically reconnect to a
// different server.
type ConnectionErrorNotificationKindServerMaintenance struct {
}

func (e ConnectionErrorNotificationKindServerMaintenance) Destroy() {
}
// The credentials presented for the VPN connection are not valid.
type ConnectionErrorNotificationKindUnauthenticated struct {
}

func (e ConnectionErrorNotificationKindUnauthenticated) Destroy() {
}
// A newer session for the same account has taken over on this
// server. This session is being terminated.
type ConnectionErrorNotificationKindSuperseded struct {
}

func (e ConnectionErrorNotificationKindSuperseded) Destroy() {
}

type FfiConverterConnectionErrorNotificationKind struct {}

var FfiConverterConnectionErrorNotificationKindINSTANCE = FfiConverterConnectionErrorNotificationKind{}

func (c FfiConverterConnectionErrorNotificationKind) Lift(rb RustBufferI) ConnectionErrorNotificationKind {
	return LiftFromRustBuffer[ConnectionErrorNotificationKind](c, rb)
}

func (c FfiConverterConnectionErrorNotificationKind) Lower(value ConnectionErrorNotificationKind) C.RustBuffer {
	return LowerIntoRustBuffer[ConnectionErrorNotificationKind](c, value)
}
func (FfiConverterConnectionErrorNotificationKind) Read(reader io.Reader) ConnectionErrorNotificationKind {
	id := readInt32(reader)
	switch (id) {
		case 1:
			return ConnectionErrorNotificationKindUnknown{
				FfiConverterInt32INSTANCE.Read(reader),
			};
		case 2:
			return ConnectionErrorNotificationKindConnectionLimitReached{
			};
		case 3:
			return ConnectionErrorNotificationKindServerMaintenance{
			};
		case 4:
			return ConnectionErrorNotificationKindUnauthenticated{
			};
		case 5:
			return ConnectionErrorNotificationKindSuperseded{
			};
		default:
			panic(fmt.Sprintf("invalid enum value %v in FfiConverterConnectionErrorNotificationKind.Read()", id));
	}
}

func (FfiConverterConnectionErrorNotificationKind) Write(writer io.Writer, value ConnectionErrorNotificationKind) {
	switch variant_value := value.(type) {
		case ConnectionErrorNotificationKindUnknown:
			writeInt32(writer, 1)
			FfiConverterInt32INSTANCE.Write(writer, variant_value.Kind)
		case ConnectionErrorNotificationKindConnectionLimitReached:
			writeInt32(writer, 2)
		case ConnectionErrorNotificationKindServerMaintenance:
			writeInt32(writer, 3)
		case ConnectionErrorNotificationKindUnauthenticated:
			writeInt32(writer, 4)
		case ConnectionErrorNotificationKindSuperseded:
			writeInt32(writer, 5)
		default:
			_ = variant_value
			panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterConnectionErrorNotificationKind.Write", value))
	}
}

type FfiDestroyerConnectionErrorNotificationKind struct {}

func (_ FfiDestroyerConnectionErrorNotificationKind) Destroy(value ConnectionErrorNotificationKind) {
	value.Destroy()
}




// Which VPN protocol the accompanying `Credentials` are for. Reserved
// for future protocol-specific behavior; currently the library treats
// both variants identically on the wire.
type CredentialsKind uint

const (
	CredentialsKindOpenVpn CredentialsKind = 1
	CredentialsKindNordWhisper CredentialsKind = 2
)

type FfiConverterCredentialsKind struct {}

var FfiConverterCredentialsKindINSTANCE = FfiConverterCredentialsKind{}

func (c FfiConverterCredentialsKind) Lift(rb RustBufferI) CredentialsKind {
	return LiftFromRustBuffer[CredentialsKind](c, rb)
}

func (c FfiConverterCredentialsKind) Lower(value CredentialsKind) C.RustBuffer {
	return LowerIntoRustBuffer[CredentialsKind](c, value)
}
func (FfiConverterCredentialsKind) Read(reader io.Reader) CredentialsKind {
	id := readInt32(reader)
	return CredentialsKind(id)
}

func (FfiConverterCredentialsKind) Write(writer io.Writer, value CredentialsKind) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerCredentialsKind struct {}

func (_ FfiDestroyerCredentialsKind) Destroy(value CredentialsKind) {
}


// Errors returned by ENS library operations.
//
// Each variant carries a human-readable reason string. Reason strings are
// censored in release builds to remove sensitive data (e.g. credentials,
// server addresses) before they reach the caller's logs.
type EnsError struct {
	err error
}

// Convience method to turn *EnsError into error
// Avoiding treating nil pointer as non nil error interface
func (err *EnsError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err EnsError) Error() string {
	return fmt.Sprintf("EnsError: %s", err.err.Error())
}

func (err EnsError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrEnsErrorTransportError = fmt.Errorf("EnsErrorTransportError")
var ErrEnsErrorStatusError = fmt.Errorf("EnsErrorStatusError")
var ErrEnsErrorInternalError = fmt.Errorf("EnsErrorInternalError")
var ErrEnsErrorNotInitialized = fmt.Errorf("EnsErrorNotInitialized")
var ErrEnsErrorAlreadyInitialized = fmt.Errorf("EnsErrorAlreadyInitialized")
var ErrEnsErrorUnknownError = fmt.Errorf("EnsErrorUnknownError")

// Variant structs
// Network or TLS failure while talking to the ENS server.
type EnsErrorTransportError struct {
	Reason string
}
// Network or TLS failure while talking to the ENS server.
func NewEnsErrorTransportError(
	reason string,
) *EnsError {
	return &EnsError { err: &EnsErrorTransportError {
			Reason: reason,} }
}

func (e EnsErrorTransportError) destroy() {
		FfiDestroyerString{}.Destroy(e.Reason)
}


func (err EnsErrorTransportError) Error() string {
	return fmt.Sprint("TransportError",
		": ",
		
		"Reason=",
		err.Reason,
	)
}

func (self EnsErrorTransportError) Is(target error) bool {
	return target == ErrEnsErrorTransportError
}
// The ENS server returned a gRPC status error.
type EnsErrorStatusError struct {
	Reason string
}
// The ENS server returned a gRPC status error.
func NewEnsErrorStatusError(
	reason string,
) *EnsError {
	return &EnsError { err: &EnsErrorStatusError {
			Reason: reason,} }
}

func (e EnsErrorStatusError) destroy() {
		FfiDestroyerString{}.Destroy(e.Reason)
}


func (err EnsErrorStatusError) Error() string {
	return fmt.Sprint("StatusError",
		": ",
		
		"Reason=",
		err.Reason,
	)
}

func (self EnsErrorStatusError) Is(target error) bool {
	return target == ErrEnsErrorStatusError
}
// The library reached a state it does not know how to recover from.
type EnsErrorInternalError struct {
	Reason string
}
// The library reached a state it does not know how to recover from.
func NewEnsErrorInternalError(
	reason string,
) *EnsError {
	return &EnsError { err: &EnsErrorInternalError {
			Reason: reason,} }
}

func (e EnsErrorInternalError) destroy() {
		FfiDestroyerString{}.Destroy(e.Reason)
}


func (err EnsErrorInternalError) Error() string {
	return fmt.Sprint("InternalError",
		": ",
		
		"Reason=",
		err.Reason,
	)
}

func (self EnsErrorInternalError) Is(target error) bool {
	return target == ErrEnsErrorInternalError
}
// A library function was called before `init()`, or after
// `deinit()`. Call `init()` before any other function.
type EnsErrorNotInitialized struct {
	Reason string
}
// A library function was called before `init()`, or after
// `deinit()`. Call `init()` before any other function.
func NewEnsErrorNotInitialized(
	reason string,
) *EnsError {
	return &EnsError { err: &EnsErrorNotInitialized {
			Reason: reason,} }
}

func (e EnsErrorNotInitialized) destroy() {
		FfiDestroyerString{}.Destroy(e.Reason)
}


func (err EnsErrorNotInitialized) Error() string {
	return fmt.Sprint("NotInitialized",
		": ",
		
		"Reason=",
		err.Reason,
	)
}

func (self EnsErrorNotInitialized) Is(target error) bool {
	return target == ErrEnsErrorNotInitialized
}
// `init()` called after previous `init()` without `deinit()` in between.
type EnsErrorAlreadyInitialized struct {
}
// `init()` called after previous `init()` without `deinit()` in between.
func NewEnsErrorAlreadyInitialized(
) *EnsError {
	return &EnsError { err: &EnsErrorAlreadyInitialized {} }
}

func (e EnsErrorAlreadyInitialized) destroy() {
}


func (err EnsErrorAlreadyInitialized) Error() string {
	return fmt.Sprint("AlreadyInitialized",
		
	)
}

func (self EnsErrorAlreadyInitialized) Is(target error) bool {
	return target == ErrEnsErrorAlreadyInitialized
}
// Other, unspecified error
type EnsErrorUnknownError struct {
	Reason string
}
// Other, unspecified error
func NewEnsErrorUnknownError(
	reason string,
) *EnsError {
	return &EnsError { err: &EnsErrorUnknownError {
			Reason: reason,} }
}

func (e EnsErrorUnknownError) destroy() {
		FfiDestroyerString{}.Destroy(e.Reason)
}


func (err EnsErrorUnknownError) Error() string {
	return fmt.Sprint("UnknownError",
		": ",
		
		"Reason=",
		err.Reason,
	)
}

func (self EnsErrorUnknownError) Is(target error) bool {
	return target == ErrEnsErrorUnknownError
}

type FfiConverterEnsError struct{}

var FfiConverterEnsErrorINSTANCE = FfiConverterEnsError{}

func (c FfiConverterEnsError) Lift(eb RustBufferI) *EnsError {
	return LiftFromRustBuffer[*EnsError](c, eb)
}

func (c FfiConverterEnsError) Lower(value *EnsError) C.RustBuffer {
	return LowerIntoRustBuffer[*EnsError](c, value)
}

func (c FfiConverterEnsError) Read(reader io.Reader) *EnsError {
	errorID := readUint32(reader)

	switch errorID {
	case 1:
		return &EnsError{ &EnsErrorTransportError{
			Reason: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 2:
		return &EnsError{ &EnsErrorStatusError{
			Reason: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 3:
		return &EnsError{ &EnsErrorInternalError{
			Reason: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 4:
		return &EnsError{ &EnsErrorNotInitialized{
			Reason: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 5:
		return &EnsError{ &EnsErrorAlreadyInitialized{
		}}
	case 6:
		return &EnsError{ &EnsErrorUnknownError{
			Reason: FfiConverterStringINSTANCE.Read(reader),
		}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterEnsError.Read()", errorID))
	}
}

func (c FfiConverterEnsError) Write(writer io.Writer, value *EnsError) {
	switch variantValue := value.err.(type) {
		case *EnsErrorTransportError:
			writeInt32(writer, 1)
			FfiConverterStringINSTANCE.Write(writer, variantValue.Reason)
		case *EnsErrorStatusError:
			writeInt32(writer, 2)
			FfiConverterStringINSTANCE.Write(writer, variantValue.Reason)
		case *EnsErrorInternalError:
			writeInt32(writer, 3)
			FfiConverterStringINSTANCE.Write(writer, variantValue.Reason)
		case *EnsErrorNotInitialized:
			writeInt32(writer, 4)
			FfiConverterStringINSTANCE.Write(writer, variantValue.Reason)
		case *EnsErrorAlreadyInitialized:
			writeInt32(writer, 5)
		case *EnsErrorUnknownError:
			writeInt32(writer, 6)
			FfiConverterStringINSTANCE.Write(writer, variantValue.Reason)
		default:
			_ = variantValue
			panic(fmt.Sprintf("invalid error value `%v` in FfiConverterEnsError.Write", value))
	}
}

type FfiDestroyerEnsError struct {}

func (_ FfiDestroyerEnsError) Destroy(value *EnsError) {
	switch variantValue := value.err.(type) {
		case EnsErrorTransportError:
			variantValue.destroy()
		case EnsErrorStatusError:
			variantValue.destroy()
		case EnsErrorInternalError:
			variantValue.destroy()
		case EnsErrorNotInitialized:
			variantValue.destroy()
		case EnsErrorAlreadyInitialized:
			variantValue.destroy()
		case EnsErrorUnknownError:
			variantValue.destroy()
		default:
			_ = variantValue
			panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerEnsError.Destroy", value))
	}
}




// Which VPN protocol the accompanying `Keys` are for. Reserved for
// future protocol-specific behavior; currently only NordLynx (WireGuard)
// uses key-based ENS authentication.
type KeyKind uint

const (
	KeyKindNordLynx KeyKind = 1
)

type FfiConverterKeyKind struct {}

var FfiConverterKeyKindINSTANCE = FfiConverterKeyKind{}

func (c FfiConverterKeyKind) Lift(rb RustBufferI) KeyKind {
	return LiftFromRustBuffer[KeyKind](c, rb)
}

func (c FfiConverterKeyKind) Lower(value KeyKind) C.RustBuffer {
	return LowerIntoRustBuffer[KeyKind](c, value)
}
func (FfiConverterKeyKind) Read(reader io.Reader) KeyKind {
	id := readInt32(reader)
	return KeyKind(id)
}

func (FfiConverterKeyKind) Write(writer io.Writer, value KeyKind) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerKeyKind struct {}

func (_ FfiDestroyerKeyKind) Destroy(value KeyKind) {
}




// Log severity levels, ordered from most to least severe.
type LogLevel uint

const (
	LogLevelError LogLevel = 1
	LogLevelWarning LogLevel = 2
	LogLevelInfo LogLevel = 3
	LogLevelDebug LogLevel = 4
	LogLevelTrace LogLevel = 5
)

type FfiConverterLogLevel struct {}

var FfiConverterLogLevelINSTANCE = FfiConverterLogLevel{}

func (c FfiConverterLogLevel) Lift(rb RustBufferI) LogLevel {
	return LiftFromRustBuffer[LogLevel](c, rb)
}

func (c FfiConverterLogLevel) Lower(value LogLevel) C.RustBuffer {
	return LowerIntoRustBuffer[LogLevel](c, value)
}
func (FfiConverterLogLevel) Read(reader io.Reader) LogLevel {
	id := readInt32(reader)
	return LogLevel(id)
}

func (FfiConverterLogLevel) Write(writer io.Writer, value LogLevel) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerLogLevel struct {}

func (_ FfiDestroyerLogLevel) Destroy(value LogLevel) {
}



// Callback for receiving ENS events for a single `Connection`.
//
// All callback methods are invoked from a library-owned thread and must
// not block. Callers should hand off any nontrivial work to their own
// executor.
type ErrorNotificationCallback interface {
	
	// Delivers a connection-error notification received from the server.
	// May be called zero or more times over the life of the connection.
	Notify(notification ConnectionErrorNotification) 
	
	// Signals that the ENS session has ended and no further `notify`
	// calls will be made on this callback. Called at most once per
	// `Connection`. Calling `shutdown()` will trigger 
	// `disconnected("shutdown")`.
	Disconnected(reason *string) 
	
}


type FfiConverterCallbackInterfaceErrorNotificationCallback struct {
	handleMap *concurrentHandleMap[ErrorNotificationCallback]
}

var FfiConverterCallbackInterfaceErrorNotificationCallbackINSTANCE = FfiConverterCallbackInterfaceErrorNotificationCallback {
	handleMap: newConcurrentHandleMap[ErrorNotificationCallback](),
}

func (c FfiConverterCallbackInterfaceErrorNotificationCallback) Lift(handle uint64) ErrorNotificationCallback {
	val, ok := c.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}
	return val
}

func (c FfiConverterCallbackInterfaceErrorNotificationCallback) Read(reader io.Reader) ErrorNotificationCallback {
	return c.Lift(readUint64(reader))
}

func (c FfiConverterCallbackInterfaceErrorNotificationCallback) Lower(value ErrorNotificationCallback) C.uint64_t {
	return C.uint64_t(c.handleMap.insert(value))
}

func (c FfiConverterCallbackInterfaceErrorNotificationCallback) Write(writer io.Writer, value ErrorNotificationCallback) {
	writeUint64(writer, uint64(c.Lower(value)))
}

type FfiDestroyerCallbackInterfaceErrorNotificationCallback struct {}

func (FfiDestroyerCallbackInterfaceErrorNotificationCallback) Destroy(value ErrorNotificationCallback) {}

type uniffiCallbackResult C.int8_t

const (
	uniffiIdxCallbackFree               uniffiCallbackResult = 0
	uniffiCallbackResultSuccess         uniffiCallbackResult = 0
	uniffiCallbackResultError           uniffiCallbackResult = 1
	uniffiCallbackUnexpectedResultError uniffiCallbackResult = 2
	uniffiCallbackCancelled             uniffiCallbackResult = 3
)


type concurrentHandleMap[T any] struct {
	handles       map[uint64]T
	currentHandle uint64
	lock          sync.RWMutex
}

func newConcurrentHandleMap[T any]() *concurrentHandleMap[T] {
	return &concurrentHandleMap[T]{
		handles:  map[uint64]T{},
	}
}

func (cm *concurrentHandleMap[T]) insert(obj T) uint64 {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	cm.currentHandle = cm.currentHandle + 1
	cm.handles[cm.currentHandle] = obj
	return cm.currentHandle
}

func (cm *concurrentHandleMap[T]) remove(handle uint64) {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	delete(cm.handles, handle)
}

func (cm *concurrentHandleMap[T]) tryGet(handle uint64) (T, bool) {
	cm.lock.RLock()
	defer cm.lock.RUnlock()

	val, ok := cm.handles[handle]
	return val, ok
}

//export ens_cgo_dispatchCallbackInterfaceErrorNotificationCallbackMethod0
func ens_cgo_dispatchCallbackInterfaceErrorNotificationCallbackMethod0(uniffiHandle C.uint64_t,notification C.RustBuffer,uniffiOutReturn *C.void,callStatus *C.RustCallStatus,) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterCallbackInterfaceErrorNotificationCallbackINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}
	
	

	
    uniffiObj.Notify(
        FfiConverterConnectionErrorNotificationINSTANCE.Lift(GoRustBuffer {
		inner: notification,
	}),
    )
	
    


	
}



//export ens_cgo_dispatchCallbackInterfaceErrorNotificationCallbackMethod1
func ens_cgo_dispatchCallbackInterfaceErrorNotificationCallbackMethod1(uniffiHandle C.uint64_t,reason C.RustBuffer,uniffiOutReturn *C.void,callStatus *C.RustCallStatus,) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterCallbackInterfaceErrorNotificationCallbackINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}
	
	

	
    uniffiObj.Disconnected(
        FfiConverterOptionalStringINSTANCE.Lift(GoRustBuffer {
		inner: reason,
	}),
    )
	
    


	
}

var UniffiVTableCallbackInterfaceErrorNotificationCallbackINSTANCE = C.UniffiVTableCallbackInterfaceErrorNotificationCallback {
	notify: (C.UniffiCallbackInterfaceErrorNotificationCallbackMethod0)(C.ens_cgo_dispatchCallbackInterfaceErrorNotificationCallbackMethod0),
	disconnected: (C.UniffiCallbackInterfaceErrorNotificationCallbackMethod1)(C.ens_cgo_dispatchCallbackInterfaceErrorNotificationCallbackMethod1),

	uniffiFree: (C.UniffiCallbackInterfaceFree)(C.ens_cgo_dispatchCallbackInterfaceErrorNotificationCallbackFree),
}

//export ens_cgo_dispatchCallbackInterfaceErrorNotificationCallbackFree
func ens_cgo_dispatchCallbackInterfaceErrorNotificationCallbackFree(handle C.uint64_t) {
	FfiConverterCallbackInterfaceErrorNotificationCallbackINSTANCE.handleMap.remove(uint64(handle))
}

func (c FfiConverterCallbackInterfaceErrorNotificationCallback) register() {
	C.uniffi_ens_fn_init_callback_vtable_errornotificationcallback(&UniffiVTableCallbackInterfaceErrorNotificationCallbackINSTANCE)
}



// Callback for receiving log messages from the library.
type LogCallback interface {
	
	// Called when the library produces a log message at or below the
	// configured maximum level. Messages are censored in release builds
	// to remove sensitive data.
	Log(logLevel LogLevel, message string) 
	
}


type FfiConverterCallbackInterfaceLogCallback struct {
	handleMap *concurrentHandleMap[LogCallback]
}

var FfiConverterCallbackInterfaceLogCallbackINSTANCE = FfiConverterCallbackInterfaceLogCallback {
	handleMap: newConcurrentHandleMap[LogCallback](),
}

func (c FfiConverterCallbackInterfaceLogCallback) Lift(handle uint64) LogCallback {
	val, ok := c.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}
	return val
}

func (c FfiConverterCallbackInterfaceLogCallback) Read(reader io.Reader) LogCallback {
	return c.Lift(readUint64(reader))
}

func (c FfiConverterCallbackInterfaceLogCallback) Lower(value LogCallback) C.uint64_t {
	return C.uint64_t(c.handleMap.insert(value))
}

func (c FfiConverterCallbackInterfaceLogCallback) Write(writer io.Writer, value LogCallback) {
	writeUint64(writer, uint64(c.Lower(value)))
}

type FfiDestroyerCallbackInterfaceLogCallback struct {}

func (FfiDestroyerCallbackInterfaceLogCallback) Destroy(value LogCallback) {}



//export ens_cgo_dispatchCallbackInterfaceLogCallbackMethod0
func ens_cgo_dispatchCallbackInterfaceLogCallbackMethod0(uniffiHandle C.uint64_t,logLevel C.RustBuffer,message C.RustBuffer,uniffiOutReturn *C.void,callStatus *C.RustCallStatus,) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterCallbackInterfaceLogCallbackINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}
	
	

	
    uniffiObj.Log(
        FfiConverterLogLevelINSTANCE.Lift(GoRustBuffer {
		inner: logLevel,
	}),
        FfiConverterStringINSTANCE.Lift(GoRustBuffer {
		inner: message,
	}),
    )
	
    


	
}

var UniffiVTableCallbackInterfaceLogCallbackINSTANCE = C.UniffiVTableCallbackInterfaceLogCallback {
	log: (C.UniffiCallbackInterfaceLogCallbackMethod0)(C.ens_cgo_dispatchCallbackInterfaceLogCallbackMethod0),

	uniffiFree: (C.UniffiCallbackInterfaceFree)(C.ens_cgo_dispatchCallbackInterfaceLogCallbackFree),
}

//export ens_cgo_dispatchCallbackInterfaceLogCallbackFree
func ens_cgo_dispatchCallbackInterfaceLogCallbackFree(handle C.uint64_t) {
	FfiConverterCallbackInterfaceLogCallbackINSTANCE.handleMap.remove(uint64(handle))
}

func (c FfiConverterCallbackInterfaceLogCallback) register() {
	C.uniffi_ens_fn_init_callback_vtable_logcallback(&UniffiVTableCallbackInterfaceLogCallbackINSTANCE)
}



// Callback for protecting `socket_fd` from being routed through the VPN tunnel.
type ProtectCallback interface {
	
	Protect(socketFd int32) error
	
}


type FfiConverterCallbackInterfaceProtectCallback struct {
	handleMap *concurrentHandleMap[ProtectCallback]
}

var FfiConverterCallbackInterfaceProtectCallbackINSTANCE = FfiConverterCallbackInterfaceProtectCallback {
	handleMap: newConcurrentHandleMap[ProtectCallback](),
}

func (c FfiConverterCallbackInterfaceProtectCallback) Lift(handle uint64) ProtectCallback {
	val, ok := c.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}
	return val
}

func (c FfiConverterCallbackInterfaceProtectCallback) Read(reader io.Reader) ProtectCallback {
	return c.Lift(readUint64(reader))
}

func (c FfiConverterCallbackInterfaceProtectCallback) Lower(value ProtectCallback) C.uint64_t {
	return C.uint64_t(c.handleMap.insert(value))
}

func (c FfiConverterCallbackInterfaceProtectCallback) Write(writer io.Writer, value ProtectCallback) {
	writeUint64(writer, uint64(c.Lower(value)))
}

type FfiDestroyerCallbackInterfaceProtectCallback struct {}

func (FfiDestroyerCallbackInterfaceProtectCallback) Destroy(value ProtectCallback) {}



//export ens_cgo_dispatchCallbackInterfaceProtectCallbackMethod0
func ens_cgo_dispatchCallbackInterfaceProtectCallbackMethod0(uniffiHandle C.uint64_t,socketFd C.int32_t,uniffiOutReturn *C.void,callStatus *C.RustCallStatus,) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterCallbackInterfaceProtectCallbackINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}
	
	

	 err :=
    uniffiObj.Protect(
        FfiConverterInt32INSTANCE.Lift(socketFd),
    )
	
    
	if err != nil {
		var actualError *EnsError
		if errors.As(err, &actualError) {
			*callStatus = C.RustCallStatus {
				code: C.int8_t(uniffiCallbackResultError),
				errorBuf: FfiConverterEnsErrorINSTANCE.Lower(actualError),
			}
		} else {
			*callStatus = C.RustCallStatus {
				code: C.int8_t(uniffiCallbackUnexpectedResultError),
			}
		}
		return
	}


	
}

var UniffiVTableCallbackInterfaceProtectCallbackINSTANCE = C.UniffiVTableCallbackInterfaceProtectCallback {
	protect: (C.UniffiCallbackInterfaceProtectCallbackMethod0)(C.ens_cgo_dispatchCallbackInterfaceProtectCallbackMethod0),

	uniffiFree: (C.UniffiCallbackInterfaceFree)(C.ens_cgo_dispatchCallbackInterfaceProtectCallbackFree),
}

//export ens_cgo_dispatchCallbackInterfaceProtectCallbackFree
func ens_cgo_dispatchCallbackInterfaceProtectCallbackFree(handle C.uint64_t) {
	FfiConverterCallbackInterfaceProtectCallbackINSTANCE.handleMap.remove(uint64(handle))
}

func (c FfiConverterCallbackInterfaceProtectCallback) register() {
	C.uniffi_ens_fn_init_callback_vtable_protectcallback(&UniffiVTableCallbackInterfaceProtectCallbackINSTANCE)
}




type FfiConverterOptionalUint32 struct{}

var FfiConverterOptionalUint32INSTANCE = FfiConverterOptionalUint32{}

func (c FfiConverterOptionalUint32) Lift(rb RustBufferI) *uint32 {
	return LiftFromRustBuffer[*uint32](c, rb)
}

func (_ FfiConverterOptionalUint32) Read(reader io.Reader) *uint32 {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterUint32INSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalUint32) Lower(value *uint32) C.RustBuffer {
	return LowerIntoRustBuffer[*uint32](c, value)
}

func (_ FfiConverterOptionalUint32) Write(writer io.Writer, value *uint32) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterUint32INSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalUint32 struct {}

func (_ FfiDestroyerOptionalUint32) Destroy(value *uint32) {
	if value != nil {
		FfiDestroyerUint32{}.Destroy(*value)
	}
}



type FfiConverterOptionalString struct{}

var FfiConverterOptionalStringINSTANCE = FfiConverterOptionalString{}

func (c FfiConverterOptionalString) Lift(rb RustBufferI) *string {
	return LiftFromRustBuffer[*string](c, rb)
}

func (_ FfiConverterOptionalString) Read(reader io.Reader) *string {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterStringINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalString) Lower(value *string) C.RustBuffer {
	return LowerIntoRustBuffer[*string](c, value)
}

func (_ FfiConverterOptionalString) Write(writer io.Writer, value *string) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterStringINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalString struct {}

func (_ FfiDestroyerOptionalString) Destroy(value *string) {
	if value != nil {
		FfiDestroyerString{}.Destroy(*value)
	}
}



type FfiConverterOptionalBytes struct{}

var FfiConverterOptionalBytesINSTANCE = FfiConverterOptionalBytes{}

func (c FfiConverterOptionalBytes) Lift(rb RustBufferI) *[]byte {
	return LiftFromRustBuffer[*[]byte](c, rb)
}

func (_ FfiConverterOptionalBytes) Read(reader io.Reader) *[]byte {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterBytesINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalBytes) Lower(value *[]byte) C.RustBuffer {
	return LowerIntoRustBuffer[*[]byte](c, value)
}

func (_ FfiConverterOptionalBytes) Write(writer io.Writer, value *[]byte) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterBytesINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalBytes struct {}

func (_ FfiDestroyerOptionalBytes) Destroy(value *[]byte) {
	if value != nil {
		FfiDestroyerBytes{}.Destroy(*value)
	}
}



type FfiConverterOptionalCallbackInterfaceProtectCallback struct{}

var FfiConverterOptionalCallbackInterfaceProtectCallbackINSTANCE = FfiConverterOptionalCallbackInterfaceProtectCallback{}

func (c FfiConverterOptionalCallbackInterfaceProtectCallback) Lift(rb RustBufferI) *ProtectCallback {
	return LiftFromRustBuffer[*ProtectCallback](c, rb)
}

func (_ FfiConverterOptionalCallbackInterfaceProtectCallback) Read(reader io.Reader) *ProtectCallback {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterCallbackInterfaceProtectCallbackINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalCallbackInterfaceProtectCallback) Lower(value *ProtectCallback) C.RustBuffer {
	return LowerIntoRustBuffer[*ProtectCallback](c, value)
}

func (_ FfiConverterOptionalCallbackInterfaceProtectCallback) Write(writer io.Writer, value *ProtectCallback) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterCallbackInterfaceProtectCallbackINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalCallbackInterfaceProtectCallback struct {}

func (_ FfiDestroyerOptionalCallbackInterfaceProtectCallback) Destroy(value *ProtectCallback) {
	if value != nil {
		FfiDestroyerCallbackInterfaceProtectCallback{}.Destroy(*value)
	}
}


/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type HiddenBytes = []byte
type FfiConverterTypeHiddenBytes = FfiConverterBytes
type FfiDestroyerTypeHiddenBytes = FfiDestroyerBytes
var FfiConverterTypeHiddenBytesINSTANCE = FfiConverterBytes{}


/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type HiddenString = string
type FfiConverterTypeHiddenString = FfiConverterString
type FfiDestroyerTypeHiddenString = FfiDestroyerString
var FfiConverterTypeHiddenStringINSTANCE = FfiConverterString{}


/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type SocketAddr = string
type FfiConverterTypeSocketAddr = FfiConverterString
type FfiDestroyerTypeSocketAddr = FfiDestroyerString
var FfiConverterTypeSocketAddrINSTANCE = FfiConverterString{}

// Opens an ENS session to the VPN server at `ip:port` and starts
// monitoring for connection-error notifications. Notifications are
// delivered to `callback` on a library-owned thread.
//
// The returned `Connection` owns the background task; drop it or
// call `shutdown()` on it to end the session.
//
// The `authentication` variant must match the VPN protocol whose
// tunnel ENS is meant to monitor: `Credentials` for OpenVPN and
// NordWhisper, `Keys` for NordLynx.
func Connect(socketAddr SocketAddr, protectCallback *ProtectCallback, authentication Authentication, notificationCallback ErrorNotificationCallback, config *Config) (*Connection, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[EnsError](FfiConverterEnsError{},func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_ens_fn_func_connect(FfiConverterTypeSocketAddrINSTANCE.Lower(socketAddr), FfiConverterOptionalCallbackInterfaceProtectCallbackINSTANCE.Lower(protectCallback), FfiConverterAuthenticationINSTANCE.Lower(authentication), FfiConverterCallbackInterfaceErrorNotificationCallbackINSTANCE.Lower(notificationCallback), FfiConverterConfigINSTANCE.Lower(config),_uniffiStatus)
	})
		if _uniffiErr != nil {
			var _uniffiDefaultValue *Connection
			return _uniffiDefaultValue, _uniffiErr
		} else {
			return FfiConverterConnectionINSTANCE.Lift(_uniffiRV), nil
		}
}

// Tears down library-global state. After this returns, all other
// entry points fail with `NotInitialized` until `init()` is called
// again. Any live `Connection` handles are shut down as part of
// this call.
func Deinit() error {
	_, _uniffiErr := rustCallWithError[EnsError](FfiConverterEnsError{},func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_ens_fn_func_deinit(_uniffiStatus)
		return false
	})
		return _uniffiErr.AsError()
}

// Returns the current heap memory usage of the library in bytes.
// Intended for diagnostics.
func GetMemoryUsage() uint64 {
	return FfiConverterUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_ens_fn_func_get_memory_usage(_uniffiStatus)
	}))
}

// Returns the library version string in the format "vX.Y.Z".
func GetVersion() string {
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer {
		inner: C.uniffi_ens_fn_func_get_version(_uniffiStatus),
	}
	}))
}

// Initializes the library. The `app_version` parameter should be a succinct
// string that uniquely identifies the app that is calling the `init`
// function - it likely it should contain the app name and app version.
// Subsequent calls without a matching `deinit()` return `AlreadyInitialized`.
func Init(appVersion string) error {
	_, _uniffiErr := rustCallWithError[EnsError](FfiConverterEnsError{},func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_ens_fn_func_init(FfiConverterStringINSTANCE.Lower(appVersion),_uniffiStatus)
		return false
	})
		return _uniffiErr.AsError()
}

// Registers a callback to receive log messages. Only messages at or
// below `max_level` are forwarded. Sensitive data in log messages is
// replaced with dots in release builds. Can be called before `init` to
// enable logging output during initialisation.
//
// Replaces any previously registered log callback. Passing a new
// callback drops the previous one.
func SetLogCallback(maxLevel LogLevel, callback LogCallback) error {
	_, _uniffiErr := rustCallWithError[EnsError](FfiConverterEnsError{},func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_ens_fn_func_set_log_callback(FfiConverterLogLevelINSTANCE.Lower(maxLevel), FfiConverterCallbackInterfaceLogCallbackINSTANCE.Lower(callback),_uniffiStatus)
		return false
	})
		return _uniffiErr.AsError()
}

