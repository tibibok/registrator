#!/bin/sh
# @Author:    Konstantin Ponomarev
# @Date:      Tue Oct 03 2017
# @Email:     k.ponomarev@rsc-tech.ru
#
# Copyright (c) 2017 RSC
###


. /exmt/.functions.sh

export CONSUL_HTTP_ADDR=${CONSUL_HTTP_ADDR:-http://127.0.0.1:8500}

if ! _check consul/api -r 24h -s 3s -f silent; then
    echo "Cannot connect to Consul... Exit..."
    exit 1
fi

echo "==>"

counter=0
while [ -z $HOST_IP ]; do
    [ $counter -ge 300 ] && exit 1
    [ $counter != 0 ] && sleep 1s
    export "HOST_IP=$(curl -s ${CONSUL_HTTP_ADDR}/v1/agent/self | jq -r '.Member.Addr')"
done

set -- "-ip" "${HOST_IP}" "-resync" "${REGISTRATOR_RESYNC:-60}"
set -- /bin/registrator "$@" "consul://${CONSUL_HTTP_ADDR##*/}"

exec "$@"