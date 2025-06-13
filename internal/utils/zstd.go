package utils

import (
	"fmt"

	"github.com/klauspost/compress/zstd"
)

type ZStd struct {
	encoder *zstd.Encoder
	decoder *zstd.Decoder
}

// experiments showd best compromise with level 3
func NewZStd() (*ZStd, error) {
	encoder, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(3)))
	if err != nil {
		return nil, fmt.Errorf("zstd encoder: %v", err)
	}
	decoder, err := zstd.NewReader(nil)
	if err != nil {
		return nil, fmt.Errorf("zstd decoder: %v", err)
	}
	return &ZStd{
		encoder: encoder,
		decoder: decoder,
	}, nil
}

func (rx *ZStd) Compress(data []byte) []byte {
	return rx.encoder.EncodeAll(data, nil)
}
func (rx *ZStd) Decompress(data []byte) ([]byte, error) {
	return rx.decoder.DecodeAll(data, nil)
}
