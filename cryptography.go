package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
)

// since everyone uses the same curve its okay to have it as a global param
var eCurve elliptic.Curve = elliptic.P256()

type PrivKey struct {
	Priv *ecdsa.PrivateKey
	Pub  *PubKey
}

type PubKey struct {
	Pub   *ecdsa.PublicKey
	Bytes [32]byte // hash of x.bytes | y.bytes
}

type Sig struct {
	R *big.Int
	S *big.Int
}

func (s *Sig) bytes() []byte {
	return byteSliceAppend(s.R.Bytes(), s.S.Bytes())
}

func (k *PrivKey) gen() {
	privKey, err := ecdsa.GenerateKey(eCurve, rand.Reader)
	ifErrFatal(err, "ecdsa genkey")
	k.Priv = privKey
	k.Pub = &PubKey{}
	k.Pub.Pub = &k.Priv.PublicKey

	k.Pub.init()
}

func (k *PrivKey) sign(hashedMsg [32]byte) *Sig {
	r, s, err := ecdsa.Sign(rand.Reader, k.Priv, hashedMsg[:])
	ifErrFatal(err, "ecdsa sign")
	return &Sig{r, s}
}

func (k *PubKey) init() {
	b := k.xyBytes()
	k.Bytes = hash(b[:])
}

func (k *PubKey) verify(hashedMsg [32]byte, sig *Sig) bool {
	return verify(k.Pub, hashedMsg, sig)
}

func (k *PubKey) xyBytes() [64]byte {
	x := k.Pub.X.Bytes()
	y := k.Pub.Y.Bytes()
	b := byteSliceAppend(x, y)
	var bb [64]byte
	for i := range b {
		bb[i] = b[i]
	}
	return bb
}

func (k *PubKey) string() string {
	return hex.EncodeToString(k.Bytes[:])
}

func verify(pubKey *ecdsa.PublicKey, hashedMsg [32]byte, sig *Sig) bool {
	return ecdsa.Verify(pubKey, hashedMsg[:], sig.R, sig.S)
}

func hash(msg []byte) [32]byte {
	h := sha256.New()
	h.Write(msg)
	hashedMsg := h.Sum(nil)
	if len(hashedMsg) != 32 {
		errFatal(nil, "hash not 32 bytes")
	}
	return toByte32(hashedMsg)
}
