#!/bin/env sh

for name in styles admin; do
  npx sass --no-source-map "original-assets/styles/${name}.scss" "templates/assets/css/${name}.css"
done