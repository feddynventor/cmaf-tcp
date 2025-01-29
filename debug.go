package main

import (
	"fmt"
	"reflect"
	"strings"
	"syscall"
)

func csvLine(s syscall.TCPInfo) string {
	v := reflect.ValueOf(s)
	record := []string{}
	for i := 0; i < v.NumField(); i++ {
		record = append(record, fmt.Sprintf("%v", v.Field(i).Interface()))
	}
	return strings.Join(record, ",")
}
