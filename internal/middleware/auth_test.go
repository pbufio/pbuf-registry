package middleware

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-kratos/kratos/v2/transport"
)

type headerCarrier http.Header

func (hc headerCarrier) Get(key string) string { return http.Header(hc).Get(key) }

func (hc headerCarrier) Set(key string, value string) { http.Header(hc).Set(key, value) }

func (hc headerCarrier) Add(key string, value string) { http.Header(hc).Add(key, value) }

// Keys lists the keys stored in this carrier.
func (hc headerCarrier) Keys() []string {
	keys := make([]string, 0, len(hc))
	for k := range http.Header(hc) {
		keys = append(keys, k)
	}
	return keys
}

// Values returns a slice value associated with the passed key.
func (hc headerCarrier) Values(key string) []string {
	return http.Header(hc).Values(key)
}

func newTokenHeader(headerKey string, token string) *headerCarrier {
	header := &headerCarrier{}
	header.Set(headerKey, token)
	return header
}

type Transport struct {
	kind      transport.Kind
	endpoint  string
	operation string
	reqHeader transport.Header
}

func (tr *Transport) Kind() transport.Kind {
	return tr.kind
}

func (tr *Transport) Endpoint() string {
	return tr.endpoint
}

func (tr *Transport) Operation() string {
	return tr.operation
}

func (tr *Transport) RequestHeader() transport.Header {
	return tr.reqHeader
}

func (tr *Transport) ReplyHeader() transport.Header {
	return nil
}

func Test_noAuth_NewAuthMiddleware(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "noAuth_NewAuthMiddleware",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := noAuth{}

			auth := n.NewAuthMiddleware()
			handler := auth(func(ctx context.Context, req interface{}) (reply interface{}, err error) {
				return "hello", nil
			})

			response, err := handler(context.Background(), nil)

			if err != nil {
				t.Errorf("noAuth.NewAuthMiddleware() error = %v", err)
				return
			}

			if response != "hello" {
				t.Errorf("noAuth.NewAuthMiddleware() response = %v", response)
				return
			}
		})
	}
}

func Test_staticTokenAuth_NewAuthMiddleware(t *testing.T) {
	type fields struct {
		token string
	}
	tests := []struct {
		name    string
		fields  fields
		want    string
		wantErr bool
	}{
		{
			name: "staticTokenAuth",
			fields: fields{
				token: "simple-token",
			},
			want: "hello",
		},
		{
			name: "wrongToken",
			fields: fields{
				"wrong-token",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := staticTokenAuth{
				token: "simple-token",
			}

			auth := s.NewAuthMiddleware()

			handler := auth(func(ctx context.Context, req interface{}) (reply interface{}, err error) {
				return "hello", nil
			})

			serverContext := transport.NewServerContext(
				context.Background(),
				&Transport{reqHeader: newTokenHeader(authorizationKey, tt.fields.token)},
			)

			response, err := handler(serverContext, nil)
			if err != nil && !tt.wantErr {
				t.Errorf("staticTokenAuth.NewAuthMiddleware() error = %v", err)
				return
			}

			if response != tt.want && !tt.wantErr {
				t.Errorf("staticTokenAuth.NewAuthMiddleware() response = %v", response)
				return
			}
		})
	}
}
