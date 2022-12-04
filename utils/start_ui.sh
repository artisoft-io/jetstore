#!/bin/bash
set -e

# Version not using aws integration
# export API_DSN="postgresql://${PG_USER}:${PG_PASSWORD}@${PG_HOST}:${PG_PORT}/${PG_DATABASE}"
# echo "$API_DSN"
# apiserver -dsn "${API_DSN}" -serverAddr "${API_SERVER_ADDR}" -tokenExpiration "${API_TOKEN_EXPIRATION_MIN}"  -API_SECRET "${API_SECRET}"  -WEB_APP_DEPLOYMENT_DIR "${WEB_APP_DEPLOYMENT_DIR}"

# Version using aws integration
# Also need env JETS_LOADER_SM_ARN state machine arn
apiserver \
  -awsDsnSecret "${JETS_DSN_SECRET}" \
  -dsn "${JETS_DSN_VALUE}" \
  -awsApiSecret "${AWS_API_SECRET}" \
  -apiSecret "${API_SECRET}" \
  -awsRegion "${JETS_REGION}" \
  -serverAddr "${API_SERVER_ADDR}" \
  -tokenExpiration "${API_TOKEN_EXPIRATION_MIN}" \
  -WEB_APP_DEPLOYMENT_DIR "${WEB_APP_DEPLOYMENT_DIR}" \
  -adminEmail "${JETS_ADMIN_EMAIL}" \
  -awsAdminPwdSecret "${AWS_JETS_ADMIN_PWD_SECRET}" \
  -adminPwd "${JETS_ADMIN_PWD}" 
