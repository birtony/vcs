/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cslstore

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/hyperledger/aries-framework-go/pkg/doc/verifiable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/trustbloc/vcs/pkg/internal/testutil"
	"github.com/trustbloc/vcs/pkg/service/credentialstatus"
)

const (
	bucket   = "test-bucket"
	region   = "test-region"
	hostName = ""
)

var (
	//go:embed testdata/university_degree.jsonld
	sampleVCJsonLD string
	//go:embed testdata/university_degree.jwt
	sampleVCJWT string
)

type mockS3Uploader struct {
	t      *testing.T
	m      map[string]*s3.PutObjectInput
	putErr error
	getErr error
}

type notFoundError struct{}

func (s notFoundError) Code() string {
	return "NoSuchKey"
}

// Message returns the exception's message.
func (s notFoundError) Message() string {
	return "NoSuchKey"
}

// OrigErr always returns nil, satisfies awserr.Error interface.
func (s notFoundError) OrigErr() error {
	return nil
}

func (s notFoundError) Error() string {
	return fmt.Sprintf("%s: %s", s.Code(), s.Message())
}

func (m *mockS3Uploader) PutObject(input *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
	if m.putErr != nil {
		return nil, m.putErr
	}
	assert.Equal(m.t, "application/json", *input.ContentType)
	assert.NotEmpty(m.t, *input.Key)
	assert.Equal(m.t, bucket, *input.Bucket)
	assert.False(m.t, strings.HasPrefix(*input.Key,
		NewStore(nil, bucket, region, hostName).getAmazonPublicDomain()))
	m.m[*input.Key] = input

	return &s3.PutObjectOutput{}, nil
}

func (m *mockS3Uploader) GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	assert.NotEmpty(m.t, *input.Key)
	assert.Equal(m.t, bucket, *input.Bucket)
	assert.False(m.t, strings.HasPrefix(*input.Key,
		NewStore(nil, bucket, region, hostName).getAmazonPublicDomain()))

	putData, ok := m.m[*input.Key]
	if !ok {
		return nil, &notFoundError{}
	}

	b, err := io.ReadAll(putData.Body)
	assert.NoError(m.t, err)

	return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(b))}, nil
}

func TestWrapperStore(t *testing.T) {
	tests := []struct {
		name string
		file []byte
	}{
		{
			name: "JSON-LD",
			file: []byte(sampleVCJsonLD),
		},
		{
			name: "JWT",
			file: []byte(sampleVCJWT),
		},
	}
	for _, tt := range tests {
		t.Run("Create, update, find wrapper VC "+tt.name, func(t *testing.T) {
			client := &mockS3Uploader{m: map[string]*s3.PutObjectInput{}, t: t}
			store := NewStore(client, bucket, region, hostName)

			vc, err := verifiable.ParseCredential(tt.file,
				verifiable.WithJSONLDDocumentLoader(testutil.DocumentLoader(t)),
				verifiable.WithDisabledProofCheck())
			assert.NoError(t, err)

			vcBytes, err := vc.MarshalJSON()
			assert.NoError(t, err)

			wrapperCreated := &credentialstatus.CSLWrapper{
				VCByte:              vcBytes,
				RevocationListIndex: 1,
				VC:                  vc,
			}
			// Create - Find
			err = store.Upsert(wrapperCreated)
			assert.NoError(t, err)

			var wrapperFound *credentialstatus.CSLWrapper
			wrapperFound, err = store.Get(vc.ID)
			assert.NoError(t, err)
			compareWrappers(t, wrapperCreated, wrapperFound)

			// Update - Find
			wrapperCreated.VCByte = vcBytes
			wrapperCreated.RevocationListIndex++
			wrapperCreated.Size++
			wrapperCreated.ListID++

			err = store.Upsert(wrapperCreated)
			assert.NoError(t, err)

			wrapperFound, err = store.Get(vc.ID)
			assert.NoError(t, err)

			compareWrappers(t, wrapperCreated, wrapperFound)
		})
	}

	t.Run("Unexpected error from s3 client on upsert", func(t *testing.T) {
		errClient := &mockS3Uploader{m: map[string]*s3.PutObjectInput{}, t: t, putErr: errors.New("some error")}
		wrapperCreated := &credentialstatus.CSLWrapper{
			VC: &verifiable.Credential{ID: ""},
		}
		err := NewStore(errClient, bucket, region, hostName).Upsert(wrapperCreated)

		assert.Error(t, err)
		assert.ErrorContains(t, err, "failed to upload CSL")
	})

	t.Run("Find non-existing CSL", func(t *testing.T) {
		errClient := &mockS3Uploader{
			m: map[string]*s3.PutObjectInput{},
			t: t,
		}
		resp, err := NewStore(errClient, bucket, region, hostName).Get("http://example.gov/credentials/3732.json")

		assert.Nil(t, resp)
		assert.ErrorIs(t, err, credentialstatus.ErrDataNotFound)
	})

	t.Run("Find non-existing cslWrapper", func(t *testing.T) {
		errClient := &mockS3Uploader{
			m: map[string]*s3.PutObjectInput{
				"http://example.gov/credentials/3732.json": {Body: bytes.NewReader([]byte(``))},
			},
			t: t,
		}
		resp, err := NewStore(errClient, bucket, region, hostName).Get("http://example.gov/credentials/3732.json")

		assert.Nil(t, resp)
		assert.ErrorIs(t, err, credentialstatus.ErrDataNotFound)
	})

	t.Run("Unexpected error from s3 client on get", func(t *testing.T) {
		errClient := &mockS3Uploader{m: map[string]*s3.PutObjectInput{}, t: t, getErr: errors.New("some error")}
		resp, err := NewStore(errClient, bucket, region, hostName).Get("63451f2358bde34a13b5d95b")

		assert.Nil(t, resp)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "failed to get CSL from S3")
	})

	t.Run("Malformed data Error on get", func(t *testing.T) {
		errClient := &mockS3Uploader{
			m: map[string]*s3.PutObjectInput{
				"http://example.gov/credentials/3732.json":         {Body: bytes.NewReader([]byte(``))},
				"http://example.gov/credentials/3732_wrapper.json": {Body: bytes.NewReader([]byte(``))},
			},
			t: t,
		}
		resp, err := NewStore(errClient, bucket, region, hostName).Get("http://example.gov/credentials/3732.json")

		assert.Nil(t, resp)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "failed to decode cslWrapper")
	})
}

func TestLatestListID(t *testing.T) {
	client := &mockS3Uploader{m: map[string]*s3.PutObjectInput{}, t: t}
	store := NewStore(client, bucket, region, hostName)
	require.NotNil(t, store)

	t.Run("Find non-existing ID", func(t *testing.T) {
		id, err := store.GetLatestListID()

		assert.Equal(t, 1, id)
		assert.NoError(t, err)
	})

	t.Run("Create - Update - Get LatestListID", func(t *testing.T) {
		expectedID := 1

		receivedID, err := store.GetLatestListID()
		require.NoError(t, err)
		if !assert.Equal(t, expectedID, receivedID) {
			t.Errorf("LatestListID got = %v, want %v",
				receivedID, expectedID)
		}

		expectedID++
		err = store.UpdateLatestListID(expectedID)
		require.NoError(t, err)

		receivedID, err = store.GetLatestListID()
		require.NoError(t, err)
		if !assert.Equal(t, expectedID, receivedID) {
			t.Errorf("LatestListID got = %v, want %v",
				receivedID, expectedID)
		}
	})

	t.Run("Unexpected error from s3 client on GetLatestListID", func(t *testing.T) {
		c := &mockS3Uploader{m: map[string]*s3.PutObjectInput{}, t: t, getErr: errors.New("some error")}
		id, err := NewStore(c, bucket, region, hostName).GetLatestListID()

		assert.Equal(t, -1, id)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "failed to get latestListID from S3")
	})

	t.Run("Malformed data error on GetLatestListID", func(t *testing.T) {
		errClient := &mockS3Uploader{
			m: map[string]*s3.PutObjectInput{
				"/issuer/latestlistid/latestListID.json": {Body: bytes.NewReader([]byte(``))},
			},
			t: t,
		}
		id, err := NewStore(errClient, bucket, region, hostName).GetLatestListID()

		assert.Equal(t, -1, id)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "failed to decode latestListID")
	})

	t.Run("Unexpected error from s3 client on CreateLatestListID", func(t *testing.T) {
		c := &mockS3Uploader{m: map[string]*s3.PutObjectInput{}, t: t, putErr: errors.New("some error")}
		id, err := NewStore(c, bucket, region, hostName).GetLatestListID()

		assert.Equal(t, -1, id)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "failed to upload latestListID")
	})

	t.Run("Unexpected error from s3 client on UpdateLatestListID", func(t *testing.T) {
		c := &mockS3Uploader{m: map[string]*s3.PutObjectInput{}, t: t, putErr: errors.New("some error")}
		err := NewStore(c, bucket, region, hostName).UpdateLatestListID(1)

		assert.Error(t, err)
		assert.ErrorContains(t, err, "failed to upload latestListID")
	})
}

func TestStore_GetCSLURL(t *testing.T) {
	store := NewStore(nil, bucket, region, hostName)
	require.NotNil(t, store)

	cslURL, err := store.GetCSLURL(
		"https://example.com", "test_issuer", "1")
	assert.NoError(t, err)
	assert.Equal(t,
		"https://test-bucket.s3.test-region.amazonaws.com/issuer/profiles/test_issuer/credentials/status/1.json",
		cslURL)

	// Convert to CSL S3 key.
	cslS3Key := store.resolveCSLS3Key(cslURL)
	assert.Equal(t,
		"/issuer/profiles/test_issuer/credentials/status/1.json",
		cslS3Key)

	// Convert to cslWrapper S3 key.
	cslWrapperS3Key := store.resolveCSLWrapperS3Key(cslURL)
	assert.Equal(t,
		"/issuer/profiles/test_issuer/credentials/status/1_wrapper.json",
		cslWrapperS3Key)
}

func compareWrappers(t *testing.T, wrapperCreated, wrapperFound *credentialstatus.CSLWrapper) {
	t.Helper()

	vcFound, err := verifiable.ParseCredential(wrapperFound.VCByte,
		verifiable.WithJSONLDDocumentLoader(testutil.DocumentLoader(t)),
		verifiable.WithDisabledProofCheck())
	assert.NoError(t, err)

	if !assert.Equal(t, wrapperCreated.VC, vcFound) {
		t.Errorf("VC got = %v, want %v",
			wrapperFound, wrapperCreated)
	}
	if !assert.Equal(t, wrapperCreated.RevocationListIndex, wrapperFound.RevocationListIndex) {
		t.Errorf("RevocationListIndex got = %v, want %v",
			wrapperFound, wrapperCreated)
	}
}

func Test_unQuote(t *testing.T) {
	type args struct {
		s []byte
	}
	tests := []struct {
		name string
		args args
		want []byte
	}{
		{
			name: "OK",
			args: args{
				s: []byte(`"abc"`),
			},
			want: []byte(`abc`),
		},
		{
			name: "OK one quote",
			args: args{
				s: []byte(`abc"`),
			},
			want: []byte(`abc"`),
		},
		{
			name: "OK no quotes",
			args: args{
				s: []byte(`abc`),
			},
			want: []byte(`abc`),
		},
		{
			name: "OK empty string",
			args: args{
				s: []byte(``),
			},
			want: []byte(``),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, unQuote(tt.args.s), "unQuote(%v)", tt.args.s)
		})
	}
}
