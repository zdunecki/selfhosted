package server

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"reflect"
	"sync"
	"time"
)

type rsaKeypair struct {
	keyID string
	priv  *rsa.PrivateKey
	pub   *rsa.PublicKey
}

var secureKeys struct {
	mu  sync.RWMutex
	key *rsaKeypair
}

func initSecureKeypair() error {
	secureKeys.mu.Lock()
	defer secureKeys.mu.Unlock()

	if secureKeys.key != nil && secureKeys.key.priv != nil && secureKeys.key.pub != nil {
		return nil
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	// Simple key id (good enough for in-memory ephemeral keys).
	keyID := fmt.Sprintf("k-%d", time.Now().UnixNano())
	secureKeys.key = &rsaKeypair{
		keyID: keyID,
		priv:  priv,
		pub:   &priv.PublicKey,
	}
	return nil
}

func currentPublicKeySPKIB64() (keyID string, spkiB64 string, err error) {
	secureKeys.mu.RLock()
	k := secureKeys.key
	secureKeys.mu.RUnlock()
	if k == nil || k.pub == nil {
		return "", "", fmt.Errorf("secure keypair not initialized")
	}

	spkiDER, err := x509.MarshalPKIXPublicKey(k.pub)
	if err != nil {
		return "", "", err
	}
	return k.keyID, base64.StdEncoding.EncodeToString(spkiDER), nil
}

func decryptRSAOAEPB64(ciphertextB64 string, keyID string) (string, error) {
	secureKeys.mu.RLock()
	k := secureKeys.key
	secureKeys.mu.RUnlock()
	if k == nil || k.priv == nil {
		return "", fmt.Errorf("secure keypair not initialized")
	}
	if keyID != "" && k.keyID != keyID {
		return "", fmt.Errorf("unknown key id")
	}

	ct, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", fmt.Errorf("invalid ciphertext encoding")
	}

	pt, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, k.priv, ct, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt failed")
	}
	return string(pt), nil
}

// decryptSecureFields decrypts in-place any string fields tagged:
//   - `secure:"rsa_oaep_b64"` and optionally `secure_key:"<FieldNameWithKeyID>"`
//
// The ciphertext is expected to be base64-encoded RSA-OAEP(SHA-256).
func decryptSecureFields(ptr any) error {
	rv := reflect.ValueOf(ptr)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("decryptSecureFields expects a non-nil pointer")
	}
	v := rv.Elem()
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("decryptSecureFields expects a pointer to struct")
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		sf := t.Field(i)
		if sf.PkgPath != "" { // unexported
			continue
		}

		tag := sf.Tag.Get("secure")
		if tag != "rsa_oaep_b64" {
			continue
		}

		f := v.Field(i)
		if f.Kind() != reflect.String || !f.CanSet() {
			continue
		}
		ciphertext := f.String()
		if ciphertext == "" {
			continue
		}

		keyIDField := sf.Tag.Get("secure_key")
		keyID := ""
		if keyIDField != "" {
			kf := v.FieldByName(keyIDField)
			if kf.IsValid() && kf.Kind() == reflect.String {
				keyID = kf.String()
			}
		}

		plain, err := decryptRSAOAEPB64(ciphertext, keyID)
		if err != nil {
			return err
		}
		f.SetString(plain)
	}
	return nil
}

