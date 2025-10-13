package utils

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
)

// GenerateHostKey는 'keys' 디렉토리를 생성하고, RSA, ECDSA, Ed25519 호스트 개인 키를 생성하여 저장합니다.
// 개인 키는 OpenSSH 형식으로 암호화되어 저장됩니다.
func GenerateHostKey() error {
	const keyDir = "./keys"

	// 1. 키 디렉토리 생성
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return fmt.Errorf("failed to create keys directory: %v", err)
	}

	// 사용할 키 파일 경로 및 암호화에 사용할 비밀번호 (실제 사용 시 환경 변수 등 안전한 방법으로 관리해야 함)
	// OpenSSH 형식에서는 암호화에 비밀번호(passphrase)를 사용합니다.
	// 여기서는 예시로 "securepassphrase"를 사용하지만, 실제 서버 환경에서는 강력하고 안전하게 보관되는 패스프레이즈를 사용해야 합니다.
	keysToGenerate := []struct {
		path    string
		keyType string
	}{
		{path: keyDir + "/id_rsa", keyType: "rsa"},
		{path: keyDir + "/id_ecdsa", keyType: "ecdsa"},
		{path: keyDir + "/id_ed25519", keyType: "ed25519"},
	}

	for _, key := range keysToGenerate {
		if err := generateAndSaveKey(key.path, key.keyType); err != nil {
			return fmt.Errorf("failed to generate %s key: %v", key.keyType, err)
		}
		fmt.Printf("Successfully generated and encrypted %s key: %s\n", key.keyType, key.path)
	}

	return nil
}

// generateAndSaveKey는 지정된 유형의 개인 키를 생성하고 OpenSSH 형식으로 암호화하여 파일에 저장합니다.
func generateAndSaveKey(path string, keyType string) error {
	var privateKey interface{}
	var err error

	// 2. 개인 키 생성
	switch keyType {
	case "rsa":
		// RSA 키 생성 (4096 비트 권장)
		privateKey, err = rsa.GenerateKey(rand.Reader, 4096)
	case "ecdsa":
		// ECDSA 키 생성 (NIST P-521 곡선 사용 권장)
		privateKey, err = ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	case "ed25519":
		// Ed25519 키 생성
		_, privateKey, err = ed25519.GenerateKey(rand.Reader)
	default:
		return fmt.Errorf("unsupported key type: %s", keyType)
	}
	if err != nil {
		return fmt.Errorf("key generation failed: %v", err)
	}

	// 3. OpenSSH 형식으로 개인 키 마샬링 (암호화 포함)
	// MarshalPrivateKeyWithPassphrase는 OpenSSH 형식으로 키를 암호화합니다.
	privatePEM, err := ssh.MarshalPrivateKey(
		privateKey,
		"",
	)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %v", err)
	}

	// 4. 개인 키 파일 저장
	// 0600 권한은 소유자에게만 읽기/쓰기 권한을 부여하여 개인 키를 보호합니다.
	privateFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open private key file for writing: %v", err)
	}
	if err := pem.Encode(privateFile, privatePEM); err != nil {
		return fmt.Errorf("failed to write private key to file: %v", err)
	}

	// 선택 사항: 공개 키도 저장
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to create signer for public key: %v", err)
	}
	publicPEM := ssh.MarshalAuthorizedKey(signer.PublicKey())
	pubPath := path + ".pub"
	if err := os.WriteFile(pubPath, publicPEM, 0644); err != nil {
		return fmt.Errorf("failed to write public key to file: %v", err)
	}
	fmt.Printf("Generated public key: %s\n", pubPath)

	return nil
}

func CheckHostKey() ([]ssh.Signer, error) {
	keyFiles := []string{"./keys/id_rsa", "./keys/id_ecdsa", "./keys/id_ed25519"}

	for _, keyFile := range keyFiles {
		if _, err := os.Stat(keyFile); os.IsNotExist(err) {
			return nil, fmt.Errorf("key file %s does not exist", keyFile)
		}
	}

	var keys = make([]ssh.Signer, 0)
	for _, keyFile := range keyFiles {
		keyBytes, err := os.ReadFile(keyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read key file %s: %v", keyFile, err)
		}

		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key %s: %v", keyFile, err)
		}

		keys = append(keys, signer)
	}

	return keys, nil
}
