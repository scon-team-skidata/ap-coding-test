package main

import (
	"strings"
	"testing"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    bool
	}{
		{"on", `{"on":true}`, true},
		{"off", `{"on":false}`, false},
		{"whitespace", " \n { \"on\" : false } \t\n", false},
		{"escaped field name", `{"\u006fn":true}`, true},
		{"size limit", `{"on":true}` + strings.Repeat(" ", 1024-len(`{"on":true}`)), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCommand([]byte(tt.payload))
			if err != nil {
				t.Fatalf("parseCommand() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("parseCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseCommandRejectsInvalidPayloads(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{"empty", ""},
		{"whitespace only", " \n\t"},
		{"malformed JSON", `{"on":`},
		{"missing closing brace", `{"on":true`},
		{"wrong closing delimiter", `{"on":true]`},
		{"trailing comma", `{"on":true,}`},
		{"missing on", `{}`},
		{"null object", `null`},
		{"array", `[{"on":true}]`},
		{"scalar", `true`},
		{"null on", `{"on":null}`},
		{"string on", `{"on":"false"}`},
		{"number on", `{"on":0}`},
		{"object on", `{"on":{}}`},
		{"array on", `{"on":[]}`},
		{"unknown field", `{"toggle":true}`},
		{"extra field before on", `{"extra":1,"on":true}`},
		{"extra field after on", `{"on":false,"extra":1}`},
		{"case sensitive field", `{"On":true}`},
		{"duplicate field", `{"on":true,"on":false}`},
		{"duplicate identical field", `{"on":false,"on":false}`},
		{"escaped duplicate field", `{"on":true,"\u006fn":true}`},
		{"second object", `{"on":true}{"on":false}`},
		{"trailing scalar", `{"on":true} false`},
		{"trailing garbage", `{"on":true} junk`},
		{"over size limit", `{"on":true}` + strings.Repeat(" ", 1025-len(`{"on":true}`))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := parseCommand([]byte(tt.payload)); err == nil {
				t.Errorf("parseCommand() = %v, nil; want an error", got)
			}
		})
	}
}
