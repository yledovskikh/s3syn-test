package utils

import "crypto/md5"

func CalculateMD5(data []byte) [16]byte {
	return md5.Sum(data)
}
