package gistova

import "context"

type ContextKey string

var CtxXRayTrace ContextKey = "xraytrace"
var CtxRequestID ContextKey = "requestid"

func GetXRayTrace(ctx context.Context) string {
	v := ctx.Value(CtxXRayTrace)
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func GetRequestID(ctx context.Context) string {
	v := ctx.Value(CtxRequestID)
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
