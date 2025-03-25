package utils

import (
	"context"

	"github.com/Fl0rencess720/Wittgenstein/pkgs/jwtc"
)

func GetPhoneFromContext(ctx context.Context) string {
	phone, ok := ctx.Value(jwtc.PhoneKey).(string)
	if !ok {
		return ""
	}
	return string(phone)
}
