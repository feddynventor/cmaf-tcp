#!/bin/sh

IFACE=eth0

#EGRESS_LIMIT=5mbit
#LATENCY=100ms

if [ -z "$EGRESS_LIMIT" ]; then
  echo "EGRESS_LIMIT is not set"
  ./main
  exit 0
fi
if [ -z "$LATENCY" ]; then
  echo "LATENCY is not set"
  ./main
  exit 0
fi

# Add a root qdisc (htb - Hierarchical Token Bucket)
tc qdisc add dev $IFACE root handle 1: htb
# Create a class with a rate limit of 1Mbps for outbound traffic
tc class add dev $IFACE parent 1: classid 1:1 htb rate $EGRESS_LIMIT
# Attach a netem qdisc under the class to add latency
tc qdisc add dev $IFACE parent 1:1 handle 10: netem delay $LATENCY

# Attach a filter to the class for outbound traffic
tc filter add dev $IFACE protocol ip parent 1:0 prio 1 u32 match ip src 0.0.0.0/0 flowid 1:1

# summary
tc qdisc show dev $IFACE

echo "##########"

# run server
./main $1