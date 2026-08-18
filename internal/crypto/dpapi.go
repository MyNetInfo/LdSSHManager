//go:build windows

// Package crypto 提供本地敏感数据的加密原语。
// 目前提供 Windows DPAPI 封装: 用"当前 Windows 用户"级密钥加密/解密,
// 密钥由系统管理(不落盘、不导出), 换机器/换用户无法解密 ——
// 适合加密"只在本机使用"的敏感字段(如云端加密密码、登录 token)的落盘。
package crypto

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

// DPAPIEncrypt 用 Windows DPAPI(当前用户)加密明文, 返回密文。
// 空输入返回空结果(不调用系统 API)。
func DPAPIEncrypt(plain []byte) ([]byte, error) {
	if len(plain) == 0 {
		return nil, nil
	}
	in := windows.DataBlob{
		Data: &plain[0],
		Size: uint32(len(plain)),
	}
	var out windows.DataBlob
	if err := windows.CryptProtectData(&in, nil, nil, 0, nil, 0, &out); err != nil {
		return nil, err
	}
	defer localFree(out.Data)
	buf := make([]byte, int(out.Size))
	copy(buf, unsafe.Slice(out.Data, int(out.Size)))
	return buf, nil
}

// DPAPIDecrypt 用 Windows DPAPI(当前用户)解密密文。
// 空输入返回空结果; 密文无效(非本用户/被篡改)返回错误。
func DPAPIDecrypt(cipher []byte) ([]byte, error) {
	if len(cipher) == 0 {
		return nil, nil
	}
	in := windows.DataBlob{
		Data: &cipher[0],
		Size: uint32(len(cipher)),
	}
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(&in, nil, nil, 0, nil, 0, &out); err != nil {
		return nil, errors.New("DPAPI 解密失败(可能文件来自其它用户/机器): " + err.Error())
	}
	defer localFree(out.Data)
	buf := make([]byte, int(out.Size))
	copy(buf, unsafe.Slice(out.Data, int(out.Size)))
	return buf, nil
}

// localFree 释放 DPAPI 返回的内存(LocalFree), 忽略错误
func localFree(data *byte) {
	if data == nil {
		return
	}
	_, _ = windows.LocalFree(windows.Handle(unsafe.Pointer(data)))
}
