#!/bin/bash

# Fixture-only regression tests for the RPM checker, not crypto/runtime tests.
# The mock cross-strings reads marker text instead of inspecting target ELF files.
set -euo pipefail

ROOTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHECKER="${CHECK_FIPS_SCRIPT:-${ROOTDIR}/macros/check-fips}"
TESTDIR="$(mktemp -d)"
trap 'rm -rf "${TESTDIR}"' EXIT

REAL_FIND="$(command -v find)"
export REAL_FIND
mkdir -p "${TESTDIR}/tools"
cat > "${TESTDIR}/tools/fixture-strings" <<'EOF'
#!/bin/bash
set -euo pipefail
if [[ "${1}" == *"/strings-error"* ]]; then
    # Even apparently valid partial output must not hide the scanner failure.
    printf 'rust_alloc\naws-lc-rs\naws-lc-fips-sys\n'
    echo "fixture strings failure" >&2
    exit 23
fi
if [[ "${STRINGS_FAULT:-}" == empty ]]; then
    echo "fixture empty strings failure" >&2
    exit 23
fi
if [[ "${STRINGS_FAULT:-}" == twin && "${1}" == *"/fips/"* ]]; then
    printf 'aws-lc-fips-sys\n'
    echo "fixture twin strings failure" >&2
    exit 23
fi
cat -- "${1}"
EOF
cat > "${TESTDIR}/tools/find" <<'EOF'
#!/bin/bash
set -euo pipefail
case "${FIND_FAULT:-}" in
    before)
        echo "fixture find failure" >&2
        exit 24
        ;;
    after)
        "${REAL_FIND}" "$@"
        echo "fixture find failure after output" >&2
        exit 24
        ;;
    *)
        exec "${REAL_FIND}" "$@"
        ;;
esac
EOF
chmod +x "${TESTDIR}/tools/fixture-strings" "${TESTDIR}/tools/find"

rust=$'rust_alloc\n'
plain="${rust}"$'aws-lc-rs\n'
runtime="${plain}"$'aws-lc-fips-sys\n'
ring="${rust}"$'/ring/src\n'
passed=0
failed=0

new_case() {
    name="${1}"
    case_dir="${TESTDIR}/${name}"
    buildroot="${case_dir}/buildroot"
    bin="${buildroot}/usr/bin"
    fips_bin="${buildroot}/usr/fips/bin"
    libexec="${buildroot}/usr/libexec"
    fips_libexec="${buildroot}/usr/fips/libexec"
    bindir="/usr/bin"
    fips_bindir="/usr/fips/bin"
    libexecdir="/usr/libexec"
    fips_libexecdir="/usr/fips/libexec"
    find_fault=""
    strings_fault=""
    mkdir -p "${case_dir}"
}

binary() {
    mkdir -p "$(dirname "${1}")"
    printf '%s' "${2}" > "${1}"
    chmod +x "${1}"
}

run_case() {
    local expected="${1}" pattern="${2}" rc=0
    PATH="${TESTDIR}/tools:${PATH}" \
        RPM_BUILD_ROOT="${buildroot}" FIND_FAULT="${find_fault}" \
        STRINGS_FAULT="${strings_fault}" \
        /bin/bash "${CHECKER}" \
        --target=fixture --bindir="${bindir}" --fips-bindir="${fips_bindir}" \
        --libexecdir="${libexecdir}" --fips-libexecdir="${fips_libexecdir}" \
        > "${case_dir}/stdout" 2> "${case_dir}/stderr" || rc=$?

    if { [[ "${expected}" == pass && "${rc}" -eq 0 ]] ||
         [[ "${expected}" == fail && "${rc}" -ne 0 ]]; } &&
        { [[ "${expected}" != pass ]] || [[ ! -s "${case_dir}/stderr" ]]; } &&
        { [[ -z "${pattern}" ]] ||
          grep -Fq -- "${pattern}" "${case_dir}/stdout" "${case_dir}/stderr"; }; then
        printf 'PASS %s (exit %s)\n' "${name}" "${rc}"
        ((passed+=1))
    else
        printf 'FAIL %s (expected %s, exit %s, diagnostic %q)\n' \
            "${name}" "${expected}" "${rc}" "${pattern}"
        cat "${case_dir}/stdout" "${case_dir}/stderr"
        ((failed+=1))
    fi
}

new_case legacy-bin-pair
binary "${bin}/client" "${plain}"
binary "${fips_bin}/client" "${runtime}"
run_case pass "Found 1 binary"

new_case legacy-libexec-pair
binary "${libexec}/plugins/client" "${plain}"
binary "${fips_libexec}/plugins/client" "${runtime}"
run_case pass "Found 1 binary"

new_case runtime-bin-without-twin
binary "${bin}/client" "${runtime}"
run_case pass "Found 1 binary"

new_case runtime-libexec-without-twin
binary "${libexec}/plugins/client" "${runtime}"
run_case pass "Found 1 binary"

new_case runtime-valid-twin
binary "${bin}/client" "${runtime}"
binary "${fips_bin}/client" "${runtime}"
run_case pass "Found 1 binary"

new_case runtime-malformed-twin
binary "${bin}/client" "${runtime}"
binary "${fips_bin}/client" "${plain}"
run_case fail "rather than in FIPS mode"

new_case legacy-missing-twin
binary "${bin}/client" "${plain}"
run_case fail "no FIPS build found"

new_case legacy-malformed-twin
binary "${bin}/client" "${plain}"
binary "${fips_bin}/client" "${plain}"
run_case fail "rather than in FIPS mode"

new_case legacy-empty-twin
binary "${bin}/client" "${plain}"
binary "${fips_bin}/client" ""
run_case fail "not a nonempty regular FIPS binary"

new_case runtime-empty-twin
binary "${bin}/client" "${runtime}"
binary "${fips_bin}/client" ""
run_case fail "not a nonempty regular FIPS binary"

new_case runtime-directory-twin
binary "${bin}/client" "${runtime}"
mkdir -p "${fips_bin}/client"
run_case fail "not a nonempty regular FIPS binary"

new_case runtime-dangling-twin
binary "${bin}/client" "${runtime}"
mkdir -p "${fips_bin}"
ln -s missing "${fips_bin}/client"
run_case fail "not a nonempty regular FIPS binary"

new_case legacy-symlink-twin
binary "${bin}/client" "${plain}"
binary "${buildroot}/fips-target" "${runtime}"
mkdir -p "${fips_bin}"
ln -s "${buildroot}/fips-target" "${fips_bin}/client"
run_case pass "Found 1 binary"

new_case ring-only
binary "${bin}/client" "${ring}"
run_case fail "must use aws-lc-rs"

new_case legacy-aws-lc-and-ring
binary "${bin}/client" "${plain}${ring}"
binary "${fips_bin}/client" "${runtime}"
run_case pass "Found 1 binary"

new_case runtime-aws-lc-and-ring
binary "${bin}/client" "${runtime}${ring}"
run_case pass "Found 1 binary"

new_case noncrypto-rust
binary "${bin}/client" "${rust}"
run_case pass ""

new_case non-rust-markers-unchanged
binary "${bin}/client" $'aws-lc-rs\n/ring/src\n'
run_case pass ""

new_case no-scan-directories
run_case pass "skipping FIPS check"

new_case both-directories
binary "${bin}/client" "${runtime}"
binary "${libexec}/plugins/client" "${plain}"
binary "${fips_libexec}/plugins/client" "${runtime}"
run_case pass "Found 2 binaries"

new_case regular-strings-error
binary "${bin}/strings-error" "${runtime}"
run_case fail "fixture strings failure"

new_case noncrypto-strings-error
binary "${bin}/strings-error" "${rust}"
run_case fail "fixture strings failure"

new_case empty-strings-error
binary "${bin}/client" "${rust}"
strings_fault=empty
run_case fail "failed to read strings"

new_case twin-strings-error
binary "${bin}/client" "${plain}"
binary "${fips_bin}/client" "${runtime}"
strings_fault=twin
run_case fail "fixture twin strings failure"

new_case find-error-before-output
binary "${bin}/client" "${runtime}"
find_fault=before
run_case fail "fixture find failure"

new_case find-error-after-output
binary "${bin}/client" "${runtime}"
find_fault=after
run_case fail "fixture find failure after output"

printf 'RESULT %s passed, %s failed\n' "${passed}" "${failed}"
[[ "${failed}" -eq 0 ]]
