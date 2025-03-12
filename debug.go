package main

import (
	"fmt"
	"reflect"
	"strings"

	"golang.org/x/sys/unix"
)

func csvLine(s unix.TCPInfo) string {
	v := reflect.ValueOf(s)
	t := reflect.TypeOf(s)

	record := []string{}
	for i := 0; i < v.NumField(); i++ {
		switch t.Field(i).Name {
		case "Rto", "Snd_mss", "Unacked", "Retrans", "Last_data_recv", "Rtt", "Rttvar", "Snd_cwnd", "Pacing_rate", "Bytes_acked", "Notsent_bytes", "Min_rtt", "Delivery_rate", "Busy_time", "Delivered", "Bytes_sent", "Snd_wnd":
			record = append(record, fmt.Sprintf("%v", v.Field(i).Interface()))
		default:
			break
		}
	}
	return strings.Join(record, ",")
}
