#!/usr/bin/env bash
set -euo pipefail

# Downloads pinned versions of hls.js, Leaflet and MarkerCluster and updates
# the version query params in the JS files so the served assets match.

LEAFLET_VERSION="1.9.4"
MARKERCLUSTER_VERSION="1.5.3"
HLSJS_VERSION="1.4.14"

# Directories to place downloaded files
LEAFLET_DIR="leaflet"
HLSJS_DIR="hlsjs"
TEMPLATES_JS_DIR="templates/assets/js"

mkdir -p "$LEAFLET_DIR" "$HLSJS_DIR"

_download() {
  local url="$1" dest="$2"
  echo "Downloading $url -> $dest"
  curl --fail --location --retry 3 --retry-delay 2 --silent --show-error -o "$dest" "$url"

  # If this is a JS file, remove any sourceMappingURL comment lines
  # without leaving an empty line where the comment was.
  if [[ "$dest" == *.js ]]; then
    if [ -f "$dest" ]; then
      tmp="$(mktemp "${TMPDIR:-/tmp}/remove-sourcemap.XXXXXX")" || return
      # Filter out lines that start with optional whitespace then '//# sourceMappingURL='
      awk '!/^[ \t]*\/\/# sourceMappingURL=/' "$dest" > "$tmp"
      # Preserve permissions and replace original file
      chmod --reference="$dest" "$tmp" 2>/dev/null || true
      mv -f "$tmp" "$dest"
    fi
  fi
}

# Download Leaflet JS and CSS
_download "https://unpkg.com/leaflet@${LEAFLET_VERSION}/dist/leaflet.js" "${LEAFLET_DIR}/leaflet.js"
_download "https://unpkg.com/leaflet@${LEAFLET_VERSION}/dist/leaflet.css" "${LEAFLET_DIR}/leaflet.css"

# Download Leaflet MarkerCluster plugin files
_download "https://unpkg.com/leaflet.markercluster@${MARKERCLUSTER_VERSION}/dist/leaflet.markercluster.js" "${LEAFLET_DIR}/markercluster.js"
_download "https://unpkg.com/leaflet.markercluster@${MARKERCLUSTER_VERSION}/dist/MarkerCluster.css" "${LEAFLET_DIR}/markercluster.css"
_download "https://unpkg.com/leaflet.markercluster@${MARKERCLUSTER_VERSION}/dist/MarkerCluster.Default.css" "${LEAFLET_DIR}/markercluster.default.css"

# Download hls.js
_download "https://unpkg.com/hls.js@${HLSJS_VERSION}/dist/hls.min.js" "${HLSJS_DIR}/hls.js"

# Remove sourceMappingURL comment lines from JS files in the downloaded files.


# Update version query params inside the JS files so they reference the
# downloaded versions. This keeps the served HTML/JS in sync with the bundled assets.

if [ -f "${TEMPLATES_JS_DIR}/video.js" ]; then
  echo "Updating ${TEMPLATES_JS_DIR}/video.js with hls.js version ${HLSJS_VERSION}"
  sed -E -i "s|(/-/hlsjs/hls\.js\?v=)[^'\"[:space:]]+|\\1${HLSJS_VERSION}|g" "${TEMPLATES_JS_DIR}/video.js"
fi

if [ -f "${TEMPLATES_JS_DIR}/geomap.js" ]; then
  echo "Updating ${TEMPLATES_JS_DIR}/geomap.js with leaflet ${LEAFLET_VERSION} and markercluster ${MARKERCLUSTER_VERSION}"
  sed -E -i \
    -e "s|(/-/leaflet/leaflet\.css\?v=)[^'\"[:space:]]+|\\1${LEAFLET_VERSION}|g" \
    -e "s|(/-/leaflet/leaflet\.js\?v=)[^'\"[:space:]]+|\\1${LEAFLET_VERSION}|g" \
    -e "s|(/-/leaflet/markercluster\.css\?v=)[^'\"[:space:]]+|\\1${MARKERCLUSTER_VERSION}|g" \
    -e "s|(/-/leaflet/markercluster\.default\.css\?v=)[^'\"[:space:]]+|\\1${MARKERCLUSTER_VERSION}|g" \
    -e "s|(/-/leaflet/markercluster\.js\?v=)[^'\"[:space:]]+|\\1${MARKERCLUSTER_VERSION}|g" \
    "${TEMPLATES_JS_DIR}/geomap.js"
fi

cat <<EOF
Finished downloading assets and updating templates:
  leaflet version: ${LEAFLET_VERSION}
  markercluster version: ${MARKERCLUSTER_VERSION}
  hls.js version: ${HLSJS_VERSION}
Files placed in:
  ${HLSJS_DIR}/
  ${LEAFLET_DIR}/
JS files updated:
  ${TEMPLATES_JS_DIR}/video.js (hls.js)
  ${TEMPLATES_JS_DIR}/geomap.js (leaflet, markercluster)
EOF