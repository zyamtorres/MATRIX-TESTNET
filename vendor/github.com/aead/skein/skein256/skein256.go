// Copyright 2018 The MATRIX Authors as well as Copyright 2014-2017 The go-ethereum Authors
// This file is consisted of the MATRIX library and part of the go-ethereum library.
//
// The MATRIX-ethereum library is free software: you can redistribute it and/or modify it under the terms of the MIT License.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"),
// to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, 
//and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject tothe following conditions:
//
//The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.
//
//THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
//FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, 
//WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISINGFROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE
//OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
// Copyright (c) 2016 Andreas Auernhammer. All rights reserved.
// Use of this source code is governed by a license that can be
// found in the LICENSE file.

package skein256

import (
	"github.com/aead/skein"
	"github.com/aead/skein/threefish"
)

type hashFunc struct {
	hashsize      int
	hVal, hValCpy [5]uint64
	tweak         [3]uint64
	block         [threefish.BlockSize256]byte
	off           int
	hasMsg        bool
}

func (s *hashFunc) BlockSize() int { return threefish.BlockSize256 }

func (s *hashFunc) Size() int { return s.hashsize }

func (s *hashFunc) Reset() {
	for i := range s.block {
		s.block[i] = 0
	}
	s.off = 0
	s.hasMsg = false

	s.hVal = s.hValCpy

	s.tweak[0] = 0
	s.tweak[1] = skein.CfgMessage<<56 | skein.FirstBlock
}

func (s *hashFunc) Write(p []byte) (n int, err error) {
	s.hasMsg = true

	n = len(p)
	var block [4]uint64

	dif := threefish.BlockSize256 - s.off
	if s.off > 0 && n > dif {
		s.off += copy(s.block[s.off:], p[:dif])
		p = p[dif:]
		if s.off == threefish.BlockSize256 && len(p) > 0 {
			bytesToBlock(&block, s.block[:])
			s.update(&block)
			s.off = 0
		}
	}

	if length := len(p); length > threefish.BlockSize256 {
		nn := length & (^(threefish.BlockSize256 - 1)) // length -= (length % BlockSize)
		if length == nn {
			nn -= threefish.BlockSize256
		}
		for i := 0; i < len(p[:nn]); i += threefish.BlockSize256 {
			bytesToBlock(&block, p[i:])
			s.update(&block)
		}
		p = p[nn:]
	}

	if len(p) > 0 {
		s.off += copy(s.block[s.off:], p)
	}
	return
}

func (s *hashFunc) Sum(b []byte) []byte {
	s0 := *s // copy

	if s0.hasMsg {
		s0.finalizeHash()
	}

	var out [threefish.BlockSize256]byte
	var ctr uint64

	for i := s0.hashsize; i > 0; i -= threefish.BlockSize256 {
		s0.output(&out, ctr)
		ctr++
		b = append(b, out[:]...)
	}

	return b[:s0.hashsize]
}

func (s *hashFunc) update(block *[4]uint64) {
	threefish.IncrementTweak(&(s.tweak), threefish.BlockSize256)

	threefish.UBI256(block, &(s.hVal), &(s.tweak))

	s.tweak[1] &^= skein.FirstBlock
}

func (s *hashFunc) output(dst *[threefish.BlockSize256]byte, counter uint64) {
	var block [4]uint64
	block[0] = counter

	hVal := s.hVal
	var outTweak = [3]uint64{8, skein.CfgOutput<<56 | skein.FirstBlock | skein.FinalBlock, 0}

	threefish.UBI256(&block, &hVal, &outTweak)
	block[0] ^= counter

	blockToBytes(dst[:], &block)
}

func (s *hashFunc) initialize(hashsize int, conf *skein.Config) {
	if hashsize < 1 {
		panic("skein256: invalid hashsize for Skein-256")
	}

	s.hashsize = hashsize

	var key, pubKey, keyID, nonce, personal []byte
	if conf != nil {
		key = conf.Key
		pubKey = conf.PublicKey
		keyID = conf.KeyID
		nonce = conf.Nonce
		personal = conf.Personal
	}

	if len(key) > 0 {
		s.tweak[0] = 0
		s.tweak[1] = skein.CfgKey<<56 | skein.FirstBlock
		s.Write(key)
		s.finalizeHash()
	}

	var cfg [32]byte
	schemaId := skein.SchemaID
	cfg[0] = byte(schemaId)
	cfg[1] = byte(schemaId >> 8)
	cfg[2] = byte(schemaId >> 16)
	cfg[3] = byte(schemaId >> 24)
	cfg[4] = byte(schemaId >> 32)
	cfg[5] = byte(schemaId >> 40)
	cfg[6] = byte(schemaId >> 48)
	cfg[7] = byte(schemaId >> 56)

	bits := uint64(s.hashsize * 8)
	cfg[8] = byte(bits)
	cfg[9] = byte(bits >> 8)
	cfg[10] = byte(bits >> 16)
	cfg[11] = byte(bits >> 24)
	cfg[12] = byte(bits >> 32)
	cfg[13] = byte(bits >> 40)
	cfg[14] = byte(bits >> 48)
	cfg[15] = byte(bits >> 56)

	s.tweak[0] = 0
	s.tweak[1] = skein.CfgConfig<<56 | skein.FirstBlock
	s.Write(cfg[:])
	s.finalizeHash()

	if len(personal) > 0 {
		s.tweak[0] = 0
		s.tweak[1] = skein.CfgPersonal<<56 | skein.FirstBlock
		s.Write(personal)
		s.finalizeHash()
	}

	if len(pubKey) > 0 {
		s.tweak[0] = 0
		s.tweak[1] = skein.CfgPublicKey<<56 | skein.FirstBlock
		s.Write(pubKey)
		s.finalizeHash()
	}

	if len(keyID) > 0 {
		s.tweak[0] = 0
		s.tweak[1] = skein.CfgKeyID<<56 | skein.FirstBlock
		s.Write(keyID)
		s.finalizeHash()
	}

	if len(nonce) > 0 {
		s.tweak[0] = 0
		s.tweak[1] = skein.CfgNonce<<56 | skein.FirstBlock
		s.Write(nonce)
		s.finalizeHash()
	}

	s.hValCpy = s.hVal

	s.Reset()
}

func (s *hashFunc) finalizeHash() {
	threefish.IncrementTweak(&(s.tweak), uint64(s.off))
	s.tweak[1] |= skein.FinalBlock // set the last block flag

	for i := s.off; i < len(s.block); i++ {
		s.block[i] = 0
	}
	s.off = 0

	var block [4]uint64
	bytesToBlock(&block, s.block[:])

	threefish.UBI256(&block, &(s.hVal), &(s.tweak))
}