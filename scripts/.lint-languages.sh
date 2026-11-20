#!/usr/bin/env bash
# DESC: Declarative language -> formatter, checker, and linter registry for lint-and-format.sh
#
# Schema:
#   register_language "<lang>" "<exts>" "<shebangs>" "<formatters>" "<checkers>" "<linters>"
#
# Tool spec format (multiple fallbacks/tools separated by ';'):
#   formatters : "bin|format_cmd|check_cmd|health_cmd; ..."  (first healthy tool wins)
#   checkers   : "bin|check_cmd|health_cmd; ..."             (all healthy tools run)
#   linters    : "bin|lint_cmd|health_cmd; ..."              (all healthy tools run)
#
# Placeholders & prefixes:
#   {file}         -> replaced safely with the target file path
#   stdout_empty:  -> passes check only if command produces empty stdout

register_language \
  "bash" \
  "sh bash" \
  "bash sh" \
  "shfmt|shfmt -w -i 2 {file}|shfmt -d -i 2 {file}|" \
  "bash|bash -n {file}|" \
  "shellcheck|shellcheck {file}|"

register_language \
  "python" \
  "py pyw" \
  "python python3" \
  "ruff|ruff format -q {file}|ruff format --check -q {file}|; black|black -q {file}|black --check -q {file}|" \
  "python3|python3 -B -m py_compile {file}|" \
  "ruff|ruff check -q {file}|; flake8|flake8 {file}|"

register_language \
  "javascript" \
  "js mjs cjs" \
  "node bun deno" \
  "prettier|prettier --write --loglevel error {file}|prettier --check --loglevel error {file}|prettier --version" \
  "node|node --check {file}|node -v" \
  "eslint|eslint --quiet {file}|eslint --version"

register_language \
  "typescript" \
  "ts tsx mts cts" \
  "tsx ts-node" \
  "prettier|prettier --write --loglevel error {file}|prettier --check --loglevel error {file}|prettier --version" \
  "" \
  "eslint|eslint --quiet {file}|eslint --version"

register_language \
  "go" \
  "go" \
  "" \
  "gofmt|gofmt -w {file}|stdout_empty:gofmt -l {file}|" \
  "go|go vet {file}|" \
  "golangci-lint|golangci-lint run {file}|"

register_language \
  "rust" \
  "rs" \
  "rust-script" \
  "rustfmt|rustfmt {file}|rustfmt --check {file}|" \
  "" \
  ""

register_language \
  "ruby" \
  "rb" \
  "ruby" \
  "rubocop|rubocop -a -f quiet {file}|rubocop -f quiet {file}|" \
  "ruby|ruby -c {file}|" \
  "rubocop|rubocop -f quiet {file}|"

register_language \
  "json" \
  "json jsonc" \
  "" \
  "prettier|prettier --write --loglevel error {file}|prettier --check --loglevel error {file}|prettier --version" \
  "python3|python3 -m json.tool {file}|" \
  ""

register_language \
  "yaml" \
  "yml yaml" \
  "" \
  "prettier|prettier --write --loglevel error {file}|prettier --check --loglevel error {file}|prettier --version" \
  "" \
  "yamllint|yamllint -d relaxed {file}|"

register_language \
  "markdown" \
  "md markdown" \
  "" \
  "prettier|prettier --write --loglevel error {file}|prettier --check --loglevel error {file}|prettier --version; mdl|mdl --fix {file}||mdl --version" \
  "" \
  "markdownlint|markdownlint {file}|markdownlint --version; mdl|mdl {file}|mdl --version"
