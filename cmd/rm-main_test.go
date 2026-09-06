// Copyright (c) 2015-2026 MinIO, Inc. and other contributors
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cmd

import (
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	rmTestBucket           = "bucket"
	rmTestVersionedObject  = "completed-object"
	rmTestIncompleteObject = "incomplete-object"
	rmTestVersionID        = "version-1"
	rmTestUploadID         = "upload-1"
)

type rmTestDelete struct {
	Key       string `xml:"Key"`
	VersionID string `xml:"VersionId"`
}

type rmTestDeleteRequest struct {
	Objects []rmTestDelete `xml:"Object"`
}

type rmTestServer struct {
	mu sync.Mutex

	versionListCalls   int
	multipartListCalls int
	deletedVersions    []rmTestDelete
	abortedUploads     []struct {
		Key      string
		UploadID string
	}
}

func (s *rmTestServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	switch {
	case r.Method == http.MethodGet && query.Has("location"):
		writeRMTestResponse(w, `<LocationConstraint xmlns="http://s3.amazonaws.com/doc/2006-03-01/"></LocationConstraint>`)
	case r.Method == http.MethodGet && query.Has("versions"):
		s.mu.Lock()
		s.versionListCalls++
		s.mu.Unlock()
		writeRMTestResponse(w, `<ListVersionsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Name>bucket</Name><Prefix></Prefix><KeyMarker></KeyMarker><VersionIdMarker></VersionIdMarker><NextKeyMarker></NextKeyMarker><NextVersionIdMarker></NextVersionIdMarker><MaxKeys>1000</MaxKeys><IsTruncated>false</IsTruncated><Version><Key>completed-object</Key><VersionId>version-1</VersionId><IsLatest>true</IsLatest><LastModified>2020-01-01T00:00:00.000Z</LastModified><ETag>&quot;etag&quot;</ETag><Size>1</Size><StorageClass>STANDARD</StorageClass></Version></ListVersionsResult>`)
	case r.Method == http.MethodGet && query.Has("uploads"):
		s.mu.Lock()
		s.multipartListCalls++
		s.mu.Unlock()
		writeRMTestResponse(w, `<ListMultipartUploadsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Bucket>bucket</Bucket><KeyMarker></KeyMarker><UploadIdMarker></UploadIdMarker><NextKeyMarker></NextKeyMarker><NextUploadIdMarker></NextUploadIdMarker><EncodingType></EncodingType><MaxUploads>1000</MaxUploads><IsTruncated>false</IsTruncated><Prefix></Prefix><Delimiter></Delimiter><Upload><Key>incomplete-object</Key><UploadId>upload-1</UploadId><Initiated>2020-01-01T00:00:00.000Z</Initiated><StorageClass>STANDARD</StorageClass></Upload></ListMultipartUploadsResult>`)
	case r.Method == http.MethodPost && query.Has("delete"):
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var request rmTestDeleteRequest
		if err = xml.Unmarshal(body, &request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		s.deletedVersions = append(s.deletedVersions, request.Objects...)
		s.mu.Unlock()
		writeRMTestResponse(w, `<DeleteResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Deleted><Key>completed-object</Key><VersionId>version-1</VersionId></Deleted></DeleteResult>`)
	case r.Method == http.MethodDelete && query.Has("uploadId"):
		s.mu.Lock()
		s.abortedUploads = append(s.abortedUploads, struct {
			Key      string
			UploadID string
		}{Key: strings.TrimPrefix(r.URL.Path, "/"), UploadID: query.Get("uploadId")})
		s.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

func writeRMTestResponse(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, body)
}

func TestListAndRemoveVersionAndIncomplete(t *testing.T) {
	tests := []struct {
		name          string
		withVersions  bool
		isIncomplete  bool
		isFake        bool
		wantVersions  bool
		wantMultipart bool
		wantDelete    bool
		wantAbort     bool
	}{
		{
			name:         "versions only",
			withVersions: true,
			wantVersions: true,
			wantDelete:   true,
		},
		{
			name:          "incomplete only",
			isIncomplete:  true,
			wantMultipart: true,
			wantAbort:     true,
		},
		{
			name:          "versions and incomplete",
			withVersions:  true,
			isIncomplete:  true,
			wantVersions:  true,
			wantMultipart: true,
			wantDelete:    true,
			wantAbort:     true,
		},
		{
			name:          "versions and incomplete dry run",
			withVersions:  true,
			isIncomplete:  true,
			isFake:        true,
			wantVersions:  true,
			wantMultipart: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			serverState := new(rmTestServer)
			server := httptest.NewServer(serverState)
			defer server.Close()

			oldAliases := aliasToConfigMap
			aliasToConfigMap = map[string]*aliasConfigV10{
				"rmtest": {
					URL:       server.URL + "/",
					AccessKey: "access-key",
					SecretKey: "secret-key",
					API:       "S3v4",
					Path:      "on",
				},
			}
			t.Cleanup(func() { aliasToConfigMap = oldAliases })

			timeRef := time.Time{}
			if test.withVersions {
				timeRef = time.Now().UTC()
			}
			err := listAndRemove("rmtest/"+rmTestBucket+"/", removeOpts{
				timeRef:      timeRef,
				withVersions: test.withVersions,
				isRecursive:  true,
				isIncomplete: test.isIncomplete,
				isFake:       test.isFake,
				isForce:      true,
			})
			if err != nil {
				t.Fatalf("listAndRemove returned error: %v", err)
			}

			serverState.mu.Lock()
			defer serverState.mu.Unlock()
			if got := serverState.versionListCalls > 0; got != test.wantVersions {
				t.Errorf("version listing called = %v, want %v", got, test.wantVersions)
			}
			if got := serverState.multipartListCalls > 0; got != test.wantMultipart {
				t.Errorf("multipart listing called = %v, want %v", got, test.wantMultipart)
			}
			if got := len(serverState.deletedVersions) > 0; got != test.wantDelete {
				t.Errorf("version deletion called = %v, want %v", got, test.wantDelete)
			}
			if got := len(serverState.abortedUploads) > 0; got != test.wantAbort {
				t.Errorf("multipart abort called = %v, want %v", got, test.wantAbort)
			}

			if test.wantDelete {
				if len(serverState.deletedVersions) != 1 {
					t.Fatalf("version deletions = %d, want 1", len(serverState.deletedVersions))
				}
				deleted := serverState.deletedVersions[0]
				if deleted.Key != rmTestVersionedObject || deleted.VersionID != rmTestVersionID {
					t.Errorf("deleted version = %#v, want key %q and version %q", deleted, rmTestVersionedObject, rmTestVersionID)
				}
			}

			if test.wantAbort {
				if len(serverState.abortedUploads) != 1 {
					t.Fatalf("multipart aborts = %d, want 1", len(serverState.abortedUploads))
				}
				aborted := serverState.abortedUploads[0]
				if aborted.Key != rmTestBucket+"/"+rmTestIncompleteObject || aborted.UploadID != rmTestUploadID {
					t.Errorf("aborted upload = %#v, want key %q and upload ID %q", aborted, rmTestBucket+"/"+rmTestIncompleteObject, rmTestUploadID)
				}
			}
		})
	}
}
