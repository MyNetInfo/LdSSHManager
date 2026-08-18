//go:build !windows

// Package crypto 提供本地敏感数据的加密原语。
// 非 Windows 平台: DPAPI 不可用, 提供空实现(调用返回错误), 由上层走 go-keyring 密钥环。
package crypto

import "errors"

// ErrDPAPINotSupported 表示当前平台不支持 Windows DPAPI。
var ErrDPAPINotSupported = errors.New("DPAPI is not supported on this platform")

// DPAPIEncrypt 非 Windows 平台不可用, 返回 ErrDPAPINotSupported。
func DPAPIEncrypt(plain []byte) ([]byte, error) {
	if len(plain) == 0 {
		return nil, nil
	}
	return nil, ErrDPAPINotSupported
}

// DPAPIDecrypt 非 Windows 平台不可用, 返回 ErrDPAPINotSupported。
func DPAPIDecrypt(ct []byte) ([]byte, error) {
	if len(ct) == 0 {
		return nil, nil
	}
	return nil, ErrDPAPINotSupported
}
