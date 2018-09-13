#!/usr/bin/env bash
#
# Copyright (c) 2018 Protocol Labs Inc.
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test API on Gateway"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon

apiport=$API_PORT

test_expect_success "GET /webui returns code expected" '
  test_curl_resp_http_code "http://127.0.0.1:$apiport/webui" "HTTP/1.1 302 Found" "HTTP/1.1 301 Moved Permanently"
'

test_expect_success "GET /webui/ returns code expected" '
  test_curl_resp_http_code "http://127.0.0.1:$apiport/webui/" "HTTP/1.1 302 Found" "HTTP/1.1 301 Moved Permanently"
'

test_expect_success "GET /logs returns logs" '
  test_expect_code 28 curl http://127.0.0.1:$apiport/logs -m1 > log_out
'

test_expect_success "log output looks good" '
  grep "log API client connected" log_out
'

test_expect_success "GET /api/v0/version succeeds" '
  curl -v "http://127.0.0.1:$apiport/api/v0/version" 2> version_out
'

test_expect_success "output only has one transfer encoding header" '
  grep "Transfer-Encoding: chunked" version_out | wc -l | xargs echo > tecount_out &&
  echo "1" > tecount_exp &&
  test_cmp tecount_out tecount_exp
'

test_kill_ipfs_daemon
test_done
