#!/usr/bin/env bash
set -e

# ==============================================================================
# vWriter Release Script
# Builds for Windows 64-bit, macOS Silicon (ARM64), and Linux 64-bit,
# creates release zips, pushes git tag, and creates GitHub Release.
# ==============================================================================

echo "================================================================="
echo "                 vWriter - Automated Release Tool               "
echo "================================================================="
echo ""

# 1. Ensure required tools exist
command -v git >/dev/null 2>&1 || { echo "[X] Error: git is required but not installed."; exit 1; }
command -v go >/dev/null 2>&1 || { echo "[X] Error: go is required but not installed."; exit 1; }

# Fetch latest tags from remote
echo "[i] Fetching remote tags..."
git fetch --tags --quiet 2>/dev/null || true

# 2. Interactive Version Input & Validation Loop
while true; do
    read -p "Enter version tag for release (e.g. v1.0.0): " VERSION
    VERSION=$(echo "$VERSION" | xargs) # trim whitespace

    if [ -z "$VERSION" ]; then
        echo "[!] Version cannot be empty."
        continue
    fi

    # Ensure 'v' prefix
    if [[ "$VERSION" != v* ]]; then
        VERSION="v$VERSION"
    fi

    # Check if tag exists locally or on remote
    LOCAL_EXISTS=$(git tag -l "$VERSION")
    REMOTE_EXISTS=$(git ls-remote --tags origin "refs/tags/$VERSION" 2>/dev/null | grep "$VERSION" || true)

    if [ -n "$LOCAL_EXISTS" ] || [ -n "$REMOTE_EXISTS" ]; then
        echo ""
        echo "[!] Tag '$VERSION' ALREADY EXISTS!"
        echo "[i] Current existing tags:"
        git tag -l | tail -n 10
        echo ""
        echo "Please specify a new version tag."
        echo ""
    else
        echo "[OK] Version '$VERSION' is valid and available."
        break
    fi
done

echo ""

# 3. GitHub Token Prompt (if not in environment)
if [ -z "$GITHUB_TOKEN" ] && [ -n "$GH_TOKEN" ]; then
    GITHUB_TOKEN="$GH_TOKEN"
fi

if [ -z "$GITHUB_TOKEN" ]; then
    read -sp "Enter your GitHub Personal Access Token: " GITHUB_TOKEN
    echo ""
    if [ -z "$GITHUB_TOKEN" ]; then
        echo "[X] GitHub token is required to publish the release."
        exit 1
    fi
fi

# Determine Repository (owner/repo)
REPO_URL=$(git remote get-url origin 2>/dev/null || true)
if [[ "$REPO_URL" =~ github\.com[:/]([^/]+)/([^/.]+)(\.git)?$ ]]; then
    REPO_OWNER="${BASH_REMATCH[1]}"
    REPO_NAME="${BASH_REMATCH[2]}"
    REPO_FULL="${REPO_OWNER}/${REPO_NAME}"
else
    REPO_FULL="jonathanhecl/vWriter"
fi

echo "[i] Target Repository: ${REPO_FULL}"
echo ""

# 4. Clean & Setup Output Directories
BUILD_DIR="dist/release_tmp"
DIST_DIR="dist"
mkdir -p "$BUILD_DIR"
mkdir -p "$DIST_DIR"

echo "================================================================="
echo "[1/4] Compiling Native Binary..."
echo "================================================================="

# Gio requires cgo + native SDK: only the host platform can be compiled here.
# Run this script on each OS to produce its binary; drop the resulting zip
# into dist/ (or build it with build_linux.sh / build_macos.sh / build.ps1).
case "$(uname -s)" in
    Linux*)                        NATIVE_OS="linux" ;;
    Darwin*)                       NATIVE_OS="darwin" ;;
    MINGW*|MSYS*|CYGWIN*)          NATIVE_OS="windows" ;;
    *)                             NATIVE_OS="unknown" ;;
esac

case "$(uname -m)" in
    x86_64|amd64)                  NATIVE_ARCH="amd64" ;;
    arm64|aarch64)                 NATIVE_ARCH="arm64" ;;
    *)                             NATIVE_ARCH="unknown" ;;
esac

echo "[i] Host platform: ${NATIVE_OS}/${NATIVE_ARCH}"

NATIVE_BIN=""
case "$NATIVE_OS" in
    windows)
        echo "[i] Compiling Windows (${NATIVE_ARCH})..."
        go build -ldflags="-s -w -H=windowsgui" -o "$BUILD_DIR/vWriter.exe" .
        NATIVE_BIN="vWriter.exe"
        ;;
    darwin)
        echo "[i] Compiling macOS (${NATIVE_ARCH})..."
        go build -ldflags="-s -w" -o "$BUILD_DIR/vWriter_mac_${NATIVE_ARCH}" .
        NATIVE_BIN="vWriter_mac_${NATIVE_ARCH}"
        ;;
    linux)
        echo "[i] Compiling Linux (${NATIVE_ARCH})..."
        go build -ldflags="-s -w" -o "$BUILD_DIR/vWriter_linux_${NATIVE_ARCH}" .
        NATIVE_BIN="vWriter_linux_${NATIVE_ARCH}"
        ;;
    *)
        echo "[X] Error: unsupported host platform '${NATIVE_OS}'."
        exit 1
        ;;
esac
echo "    -> Built: $BUILD_DIR/$NATIVE_BIN"

echo ""
echo "================================================================="
echo "[2/4] Packaging Release Archives (.zip)..."
echo "================================================================="

NATIVE_ZIP="vWriter_${VERSION}_${NATIVE_OS}_${NATIVE_ARCH}.zip"

create_zip() {
    local zip_name="$1"
    local bin_file="$2"
    local target_bin_name="$3"
    
    local tmp_pack_dir="$BUILD_DIR/pack_tmp"
    rm -rf "$tmp_pack_dir"
    mkdir -p "$tmp_pack_dir"

    cp "$BUILD_DIR/$bin_file" "$tmp_pack_dir/$target_bin_name"
    if [ -f "README.md" ]; then cp "README.md" "$tmp_pack_dir/"; fi
    if [ -f "LICENSE" ]; then cp "LICENSE" "$tmp_pack_dir/"; fi

    rm -f "$DIST_DIR/$zip_name"
    
    if command -v zip >/dev/null 2>&1; then
        (cd "$tmp_pack_dir" && zip -r -q "../../$DIST_DIR/$zip_name" .)
    elif command -v powershell.exe >/dev/null 2>&1; then
        powershell.exe -Command "Compress-Archive -Path '$tmp_pack_dir\*' -DestinationPath '$DIST_DIR/$zip_name' -Force" >/dev/null 2>&1
    elif command -v python3 >/dev/null 2>&1; then
        python3 -c "import shutil; shutil.make_archive('$DIST_DIR/${zip_name%.zip}', 'zip', '$tmp_pack_dir')"
    else
        echo "[X] Error: zip, powershell.exe or python3 is required for packaging."
        exit 1
    fi
    rm -rf "$tmp_pack_dir"
    echo "    -> Created: $DIST_DIR/$zip_name"
}

# Target binary name differs between OSes (vWriter.exe on Windows, vWriter elsewhere)
TARGET_BIN="vWriter"
[ "$NATIVE_OS" = "windows" ] && TARGET_BIN="vWriter.exe"
create_zip "$NATIVE_ZIP" "$NATIVE_BIN" "$TARGET_BIN"

rm -rf "$BUILD_DIR"

echo ""
echo "================================================================="
echo "[3/4] Release Verification"
echo "================================================================="

# Gather every vWriter_<version>_*.zip present in dist/ (native just created,
# other platforms from their own build scripts run on their host OS).
MISSING=0
ARTIFACTS=()
for zip in "$DIST_DIR"/vWriter_"${VERSION}"_*.zip; do
    if [ -f "$zip" ]; then
        SIZE=$(du -h "$zip" | cut -f1)
        echo "  [OK] $zip ($SIZE)"
        ARTIFACTS+=("$zip")
    fi
done

if [ "${#ARTIFACTS[@]}" -eq 0 ]; then
    echo "[X] No release artifacts found in '$DIST_DIR/'."
    exit 1
fi

echo ""
echo "  Platforms detected: ${#ARTIFACTS[@]} of 3 binaries."
echo "  If a platform is missing, run this script (or the matching build"
echo "  script) on that OS and copy its zip into '$DIST_DIR/'."

echo ""
echo "================================================================="
echo "[4/4] Confirmation & GitHub Release"
echo "================================================================="
echo "Version Tag : ${VERSION}"
echo "Repository  : ${REPO_FULL}"
echo "Artifacts   :"
for a in "${ARTIFACTS[@]}"; do echo "  - $a"; done
echo ""

read -p "Proceed to tag '${VERSION}', push to origin, and create GitHub Release? (y/N): " CONFIRM
if [[ "$CONFIRM" != "y" && "$CONFIRM" != "Y" ]]; then
    echo "[!] Release cancelled by user. Packages are saved in '$DIST_DIR/'."
    exit 0
fi

# Tag & Push
echo "[i] Creating Git tag '${VERSION}'..."
git tag -a "$VERSION" -m "Release $VERSION"
echo "[i] Pushing tag '${VERSION}' to remote origin..."
git push origin "$VERSION"

# GitHub Release Creation via gh CLI or REST API
if command -v gh >/dev/null 2>&1; then
    echo "[i] Creating GitHub Release using gh CLI..."
    gh release create "$VERSION" \
        "${ARTIFACTS[@]}" \
        --title "vWriter $VERSION" \
        --notes "Release $VERSION of vWriter Video Prompt Studio."
else
    echo "[i] Creating GitHub Release via GitHub API..."
    
    # Create Release
    RELEASE_PAYLOAD=$(cat <<EOF
{
  "tag_name": "$VERSION",
  "target_commitish": "main",
  "name": "vWriter $VERSION",
  "body": "Release $VERSION of vWriter Video Prompt Studio.",
  "draft": false,
  "prerelease": false
}
EOF
)

    RESPONSE=$(curl -s -X POST \
        -H "Authorization: Bearer $GITHUB_TOKEN" \
        -H "Accept: application/vnd.github.v3+json" \
        -H "Content-Type: application/json" \
        "https://api.github.com/repos/${REPO_FULL}/releases" \
        -d "$RELEASE_PAYLOAD")

    UPLOAD_URL=$(echo "$RESPONSE" | grep -o '"upload_url": "[^"]*' | cut -d'"' -f4 | sed 's/{?name,label}//')

    if [ -z "$UPLOAD_URL" ]; then
        echo "[X] Failed to create GitHub Release. API response:"
        echo "$RESPONSE"
        exit 1
    fi

    # Upload Zip Assets
    for zip in "${ARTIFACTS[@]}"; do
        echo "[i] Uploading asset: $zip..."
        curl -s -X POST \
            -H "Authorization: Bearer $GITHUB_TOKEN" \
            -H "Content-Type: application/zip" \
            "${UPLOAD_URL}?name=$(basename "$zip")" \
            --data-binary "@$zip" >/dev/null
    done
fi

echo ""
echo "================================================================="
echo " SUCCESS! Release ${VERSION} has been published to GitHub!"
echo " URL: https://github.com/${REPO_FULL}/releases/tag/${VERSION}"
echo "================================================================="
