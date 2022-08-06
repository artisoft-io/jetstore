#!/bin/bash
set -e

cd $WEB_APP_DEPLOYMENT_DIR
python3 -m http.server $UI_PORT &

export API_DSN="postgresql://${PG_USER}:${PG_PASSWORD}@${PG_HOST}:${PG_PORT}/${PG_DATABASE}"
echo "$API_DSN"
apiserver -dsn "${API_DSN}" -serverAddr "${API_SERVER_ADDR}" -tokenExpiration "${API_TOKEN_EXPIRATION_MIN}"  -API_SECRET "${API_SECRET}"
