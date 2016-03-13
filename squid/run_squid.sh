#!/usr/bin/bash

/usr/sbin/squid -f $SQUID_CONF -z && /usr/sbin/squid -f $SQUID_CONF -N
