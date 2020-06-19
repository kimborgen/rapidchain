package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"math/big"
)

// since everyone uses the same curve its okay to have it as a global param
var eCurve elliptic.Curve = elliptic.P256()

type PrivKey struct {
	Priv *ecdsa.PrivateKey
}

type PubKey struct {
	Pub *ecdsa.PublicKey
}

type Sig struct {
	R *big.Int
	S *big.Int
}

func (k *PrivKey) gen() {
	privKey, err := ecdsa.GenerateKey(eCurve, rand.Reader)
	ifErrFatal(err, "ecdsa genkey")
	k.Priv = privKey
}

func (k *PrivKey) sign(hashedMsg []byte) *Sig {
	r, s, err := ecdsa.Sign(rand.Reader, k.Priv, hashedMsg)
	ifErrFatal(err, "ecdsa sign")
	return &Sig{r, s}
}

func (k *PrivKey) pub() *PubKey {
	return &PubKey{&k.Priv.PublicKey}
}

func (k *PubKey) verify(hashedMsg []byte, sig *Sig) bool {
	return verify(k.Pub, hashedMsg, sig)
}

func (k *PubKey) bytes() []byte {
	x := getBytes(k.Pub.X)
	y := getBytes(k.Pub.Y)
	return byteSliceAppend(x, y)
}

func verify(pubKey *ecdsa.PublicKey, hashedMsg []byte, sig *Sig) bool {
	return ecdsa.Verify(pubKey, hashedMsg, sig.R, sig.S)
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
