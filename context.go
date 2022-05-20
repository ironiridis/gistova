package gistova

import "context"

type ContextKey string

var CtxRequestID ContextKey = "requestid"

func GetRequestID(ctx context.Context) string {
	v := ctx.Value(CtxRequestID)
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
