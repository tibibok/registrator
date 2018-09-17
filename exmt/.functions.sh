#!/bin/sh
# @Author:    Konstantin Ponomarev
# @Date:      Wed Oct 04 2017
# @Email:     k.ponomarev@rsc-tech.ru
#
# Copyright (c) 2017 RSC
###


function _secret() {
    path=${1?Path is not provided}
    shift
    if echo "$@" | grep '\-d|\-\-data'; then
        if ! echo "$@" | grep '\-X|\-\-request'; then
            set -- --request POST "$@"
        fi
    fi
    set -- -s --header "Content-Type:application/json" "$@"
    set -- --header "X-Vault-Token:${VAULT_TOKEN?Vault Token is not set}" "$@"
    if echo "${VAULT_ADDR:=https://vault.service.consul:8200}" | grep -q '^https:'; then
        set -- -k "$@"
    fi
    set -- "$@" ${VAULT_ADDR}/v1/${path}
    curl "$@"
}

# check with goss tool
function _check() {
    check_home=${CHECK_HOME:=/exmt/checks}
    if [ "${1}" ]; then
        path=${1}
        shift
        set -- validate "$@"
        for h in $(ls ${CHECK_HOME}/${path}.* | grep '.yml\|.json\|.yaml$'); do
            set -- -g $h "$@"
        done
    else
        set -- validate "$@"
    fi
    if [ -e "${CHECK_HOME}/vars.json" ]; then
        set -- --vars "${CHECK_HOME}/vars.json" "$@"
    fi
    goss "$@"
}

# Save various attributes to use with _check function
# @key - key to access ("password", "foo/bar")
# @value - value of attribute (always string)
function _vars() {
    check_home=${CHECK_HOME:=/exmt/checks}
    if [ ! -e "${CHECK_HOME}/vars.json" ]; then
        echo -n '{}' > "${CHECK_HOME}/vars.json"
    fi
    if [ -z "${1}" ] || [ -z "${2}" ]; then
        cat "${CHECK_HOME}/vars.json"
        return 0
    fi
    vars="$(cat "${CHECK_HOME}/vars.json" | jq -cM --arg key "${1}" --arg value "${2}" 'setpath(($key | split("/")); $value)')"
    echo -n "${vars}" > "${CHECK_HOME}/vars.json"
    chmod 0600 "${CHECK_HOME}/vars.json"
    return 0
}
