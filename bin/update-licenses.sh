#!/usr/bin/env bash
# Generates the LICENSES file for all direct dependencies listed in go.mod.
# Reads license files from the local module cache via go mod download -json.
# Indirect dependencies are excluded — they are the responsibility of their
# direct parent module.
set -euo pipefail

# Parse direct dependency module paths from go.mod.
DIRECT_DEPS=$(grep -E $'^\t[a-z]' go.mod | grep -v '// indirect' | awk '{print $1}')

if [ -z "$DIRECT_DEPS" ]; then
    echo "No direct dependencies found in go.mod." >&2
    exit 1
fi

{
    while IFS= read -r dep; do
        # Resolve module metadata from the local cache.
        mod_info=$(go list -m -json "$dep" 2>/dev/null) || { echo "warning: cannot resolve $dep, skipping" >&2; continue; }
        mod_dir=$(printf '%s' "$mod_info" | awk -F'"' '/"Dir"/{print $4; exit}')
        mod_ver=$(printf '%s' "$mod_info" | awk -F'"' '/"Version"/{print $4; exit}')

        if [ -z "$mod_dir" ] || [ ! -d "$mod_dir" ]; then
            echo "warning: no module cache dir for $dep, skipping" >&2
            continue
        fi

        # Derive the repository URL from the module path.
        # Strip trailing major-version suffix (/v2, /v8, …) and, for github.com
        # modules, keep only the owner/repo components.
        url_path=$(printf '%s' "$dep" | sed 's|/v[0-9][0-9]*$||')
        if [[ "$dep" == github.com/* ]]; then
            url="https://$(printf '%s' "$url_path" | cut -d/ -f1-3)"
        else
            url="https://$url_path"
        fi

        # Find the license file. We look only for files whose names indicate
        # license text — never source code.
        license_file=$(find "$mod_dir" -maxdepth 1 -type f \
            \( -iname "LICENSE" -o -iname "LICENSE.md" -o -iname "LICENSE.txt" \
               -o -iname "LICENCE" -o -iname "LICENCE.md" -o -iname "LICENCE.txt" \
               -o -iname "COPYING" -o -iname "COPYING.md" -o -iname "COPYING.txt" \
               -o -iname "LICENSE-MIT" -o -iname "LICENSE-APACHE" \) \
            2>/dev/null | LC_ALL=C sort | head -1)

        if [ -z "$license_file" ]; then
            echo "warning: no license file found for $dep in $mod_dir" >&2
            continue
        fi

        name=$(printf '%s' "$url" | sed 's|https://[^/]*/||')
        echo "================================================================================"
        echo "$name $mod_ver"
        echo "$url"
        echo "================================================================================"
        echo ""
        cat "$license_file"
        echo ""
    done <<< "$DIRECT_DEPS"
} > LICENSES

echo "LICENSES file updated"
