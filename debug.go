package main

import (
	"fmt"
	"reflect"
	"strings"

	"golang.org/x/sys/unix"
)

func csvLine(s unix.TCPInfo) string {
	v := reflect.ValueOf(s)
	record := []string{}
	for i := 0; i < v.NumField(); i++ {
		record = append(record, fmt.Sprintf("%v", v.Field(i).Interface()))
	}
	return strings.Join(record, ",")
}
