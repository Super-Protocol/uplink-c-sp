// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"
import (
	"reflect"
	"time"
	"unsafe"

	"storj.io/uplink"
)

// Upload is a partial upload to Storj Network.
type Upload struct {
	scope
	upload *uplink.Upload
}

//export upload_object
// upload_object starts an upload to the specified key.
func upload_object(project *C.Project, bucket_name, object_key *C.char, options *C.UploadOptions) C.UploadResult {
	if project == nil {
		return C.UploadResult{
			error: mallocError(ErrNull.New("project")),
		}
	}
	if bucket_name == nil {
		return C.UploadResult{
			error: mallocError(ErrNull.New("bucket_name")),
		}
	}
	if object_key == nil {
		return C.UploadResult{
			error: mallocError(ErrNull.New("object_key")),
		}
	}

	proj, ok := universe.Get(project._handle).(*Project)
	if !ok {
		return C.UploadResult{
			error: mallocError(ErrInvalidHandle.New("project")),
		}
	}
	scope := proj.scope.child()

	opts := &uplink.UploadOptions{}
	if options != nil {
		if options.expires > 0 {
			opts.Expires = time.Unix(int64(options.expires), 0)
		}
	}

	upload, err := proj.UploadObject(scope.ctx, C.GoString(bucket_name), C.GoString(object_key), opts)
	if err != nil {
		return C.UploadResult{
			error: mallocError(err),
		}
	}

	return C.UploadResult{
		upload: (*C.Upload)(mallocHandle(universe.Add(&Upload{scope, upload}))),
	}
}

//export upload_write
// upload_write uploads len(p) bytes from p to the object's data stream.
// It returns the number of bytes written from p (0 <= n <= len(p)) and
// any error encountered that caused the write to stop early.
func upload_write(upload *C.Upload, bytes *C.uint8_t, length C.size_t) C.WriteResult {
	// TODO: should bytes be a *void or *C.char?
	up, ok := universe.Get(upload._handle).(*Upload)
	if !ok {
		return C.WriteResult{
			error: mallocError(ErrInvalidHandle.New("upload")),
		}
	}

	ilength, ok := safeConvertToInt(length)
	if !ok {
		return C.WriteResult{
			error: mallocError(ErrInvalidArg.New("length too large")),
		}
	}

	var buf []byte
	*(*reflect.SliceHeader)(unsafe.Pointer(&buf)) = reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(bytes)),
		Len:  ilength,
		Cap:  ilength,
	}

	n, err := up.upload.Write(buf)
	return C.WriteResult{
		bytes_written: C.size_t(n),
		error:         mallocError(err),
	}
}

// TODO: should we have free_write_result?

//export upload_commit
// upload_commit commits the uploaded data.
func upload_commit(upload *C.Upload) *C.Error {
	up, ok := universe.Get(upload._handle).(*Upload)
	if !ok {
		return mallocError(ErrInvalidHandle.New("upload"))
	}

	err := up.upload.Commit()
	return mallocError(err)
}

//export upload_abort
// upload_abort aborts an upload.
func upload_abort(upload *C.Upload) *C.Error {
	up, ok := universe.Get(upload._handle).(*Upload)
	if !ok {
		return mallocError(ErrInvalidHandle.New("upload"))
	}

	err := up.upload.Abort()
	return mallocError(err)
}

//export upload_info
// upload_info returns the last information about the uploaded object.
func upload_info(upload *C.Upload) C.ObjectResult {
	up, ok := universe.Get(upload._handle).(*Upload)
	if !ok {
		return C.ObjectResult{
			error: mallocError(ErrInvalidHandle.New("upload")),
		}
	}

	info := up.upload.Info()
	return C.ObjectResult{
		object: mallocObject(info),
	}
}

//export free_write_result
// free_write_result frees any resources associated with write result.
func free_write_result(result C.WriteResult) {
	free_error(result.error)
}

//export free_upload_result
// free_upload_result closes the upload and frees any associated resources.
func free_upload_result(result C.UploadResult) *C.Error {
	free_error(result.error)
	return free_upload(result.upload)
}

//export free_upload
// free_upload closes the upload and frees any associated resources.
func free_upload(upload *C.Upload) *C.Error {
	if upload == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(upload))
	defer universe.Del(upload._handle)

	// TODO: should we return an error for invalid handle in frees?
	up, ok := universe.Get(upload._handle).(*Upload)
	if !ok {
		return mallocError(ErrInvalidHandle.New("upload"))
	}

	up.cancel()
	return nil
}
