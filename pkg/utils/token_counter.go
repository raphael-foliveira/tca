package utils

import "github.com/pkoukk/tiktoken-go"

type TokenCounter interface {
	CountTokens(text string) int64
}

type TiktokenCounter struct {
	encoding *tiktoken.Tiktoken
}

func NewTiktokenCounter() (*TiktokenCounter, error) {
	encoding, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return nil, err
	}
	return &TiktokenCounter{
		encoding: encoding,
	}, nil
}

func (t *TiktokenCounter) CountTokens(text string) int64 {
	return int64(len(t.encoding.Encode(text, nil, nil)))
}
