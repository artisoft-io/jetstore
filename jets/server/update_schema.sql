-- michel@pop-os:~/projects/repos/jetstore/jets/update_db$ JETS_DSN_SECRET=rdsSecret9F108BD1-L4s4KvRKqM6h JETS_REGION='us-east-1' JETS_BUCKET=bucket.jetstore.io WORKSPACES_HOME=/home/michel/projects/repos/artisoft-workspaces WORKSPACE=walrus_ws WORKSPACE_DB_PATH=$WORKSPACES_HOME/$WORKSPACE/workspace.db WORKSPACE_LOOKUPS_DB_PATH=$WORKSPACES_HOME/$WORKSPACE/lookup.db JETS_SCHEMA_FILE=~/projects/repos/jetstore/jets/server/jets_schema.json JETSAPI_DB_INIT_PATH=$WORKSPACES_HOME/$WORKSPACE/process_config/workspace_init_db.sql update_db -migrateDb -usingSshTunnel
-- 2023/02/12 22:28:55 Here's what we got:
-- 2023/02/12 22:28:55    -awsDsnSecret: rdsSecret9F108BD1-L4s4KvRKqM6h
-- 2023/02/12 22:28:55    -dbPoolSize: 5
-- 2023/02/12 22:28:55    -usingSshTunnel: true
-- 2023/02/12 22:28:55    -dsn len: 0
-- 2023/02/12 22:28:55    -jetsapiDbInitPath: /home/michel/projects/repos/artisoft-workspaces/walrus_ws/process_config/workspace_init_db.sql
-- 2023/02/12 22:28:55    -workspaceDb: /home/michel/projects/repos/artisoft-workspaces/walrus_ws/workspace.db
-- 2023/02/12 22:28:55    -migrateDb: true
-- 2023/02/12 22:28:55    -initWorkspaceDb: false
-- 2023/02/12 22:28:55 ENV JETSAPI_DB_INIT_PATH: /home/michel/projects/repos/artisoft-workspaces/walrus_ws/process_config/workspace_init_db.sql
-- 2023/02/12 22:28:55 ENV WORKSPACE_DB_PATH: /home/michel/projects/repos/artisoft-workspaces/walrus_ws/workspace.db
-- LOCAL TESTING using ssh tunnel (expecting ssh tunnel open)
-- 2023/02/12 22:28:56 Migrating jetsapi database to latest schema
-- Got schema for jetsapi . jetstore_release
CREATE SCHEMA IF NOT EXISTS "jetsapi"
-- Existing Table Constraints for table jetstore_release: []
ALTER TABLE IF EXISTS "jetsapi"."jetstore_release" 
ADD COLUMN IF NOT EXISTS "version" text, 
ADD COLUMN IF NOT EXISTS "name" text ;

-- Got schema for jetsapi . users
CREATE SCHEMA IF NOT EXISTS "jetsapi"
-- Existing Table Constraints for table users: []
ALTER TABLE IF EXISTS "jetsapi"."users" 
ADD COLUMN IF NOT EXISTS "user_email" text, 
ADD COLUMN IF NOT EXISTS "name" text, 
ADD COLUMN IF NOT EXISTS "password" text, 
ADD COLUMN IF NOT EXISTS "is_active" integer, 
ADD COLUMN IF NOT EXISTS "last_update" timestamp without time zone ;

-- Got schema for jetsapi . client_registry
CREATE SCHEMA IF NOT EXISTS "jetsapi"
-- Existing Table Constraints for table client_registry: []
ALTER TABLE IF EXISTS "jetsapi"."client_registry" 
ADD COLUMN IF NOT EXISTS "client" text, 
ADD COLUMN IF NOT EXISTS "details" text ;

-- Got schema for jetsapi . client_org_registry
CREATE SCHEMA IF NOT EXISTS "jetsapi"
-- Existing Table Constraints for table client_org_registry: [{client_org_registry_unique_cstraint }]
-- existingSchema.TableConstraint: {client_org_registry_unique_cstraint }
-- tableDefinition.TableConstraint: client_org_registry_unique_cstraint
ALTER TABLE IF EXISTS "jetsapi"."client_org_registry" 
ADD COLUMN IF NOT EXISTS "client" text, 
ADD COLUMN IF NOT EXISTS "org" text, 
ADD COLUMN IF NOT EXISTS "details" text ;

-- Got schema for jetsapi . mapping_function_registry
CREATE SCHEMA IF NOT EXISTS "jetsapi"
-- Existing Table Constraints for table mapping_function_registry: []
ALTER TABLE IF EXISTS "jetsapi"."mapping_function_registry" 
ADD COLUMN IF NOT EXISTS "function_name" text, 
ADD COLUMN IF NOT EXISTS "is_argument_required" integer, 
ADD COLUMN IF NOT EXISTS "details" text ;

-- Got schema for jetsapi . object_type_registry
CREATE SCHEMA IF NOT EXISTS "jetsapi"
-- Existing Table Constraints for table object_type_registry: []
ALTER TABLE IF EXISTS "jetsapi"."object_type_registry" 
ADD COLUMN IF NOT EXISTS "object_type" text, 
ADD COLUMN IF NOT EXISTS "entity_rdf_type" text, 
ADD COLUMN IF NOT EXISTS "details" text ;

-- Got schema for jetsapi . domain_keys_registry
CREATE SCHEMA IF NOT EXISTS "jetsapi"
-- Existing Table Constraints for table domain_keys_registry: []
ALTER TABLE IF EXISTS "jetsapi"."domain_keys_registry" 
ADD COLUMN IF NOT EXISTS "entity_rdf_type" text, 
ADD COLUMN IF NOT EXISTS "object_types" text ARRAY, 
ADD COLUMN IF NOT EXISTS "domain_keys_json" text ;

-- Got schema for jetsapi . object_type_mapping_details
CREATE SCHEMA IF NOT EXISTS "jetsapi"
-- Existing Table Constraints for table object_type_mapping_details: [{object_type_mapping_details_unique_cstraint }]
-- existingSchema.TableConstraint: {object_type_mapping_details_unique_cstraint }
-- tableDefinition.TableConstraint: object_type_mapping_details_unique_cstraint
ALTER TABLE IF EXISTS "jetsapi"."object_type_mapping_details" 
ADD COLUMN IF NOT EXISTS "object_type" text, 
ADD COLUMN IF NOT EXISTS "data_property" text, 
ADD COLUMN IF NOT EXISTS "is_required" integer, 
ADD COLUMN IF NOT EXISTS "details" text ;

-- Got schema for jetsapi . source_period
CREATE SCHEMA IF NOT EXISTS "jetsapi"
-- Existing Table Constraints for table source_period: [{source_period_unique_cstraint }]
-- existingSchema.TableConstraint: {source_period_unique_cstraint }
-- tableDefinition.TableConstraint: source_period_unique_cstraint
ALTER TABLE IF EXISTS "jetsapi"."source_period" 
ADD COLUMN IF NOT EXISTS "key" integer, 
ADD COLUMN IF NOT EXISTS "year" integer, 
ADD COLUMN IF NOT EXISTS "month" integer, 
ADD COLUMN IF NOT EXISTS "day" integer, 
ADD COLUMN IF NOT EXISTS "month_period" integer, 
ADD COLUMN IF NOT EXISTS "week_period" integer, 
ADD COLUMN IF NOT EXISTS "day_period" integer ;

-- Got schema for jetsapi . file_key_staging
CREATE SCHEMA IF NOT EXISTS "jetsapi"
-- Existing Table Constraints for table file_key_staging: [{file_key_staging_unique_cstraint }]
-- existingSchema.TableConstraint: {file_key_staging_unique_cstraint }
-- tableDefinition.TableConstraint: file_key_staging_unique_cstraintv2
ALTER TABLE IF EXISTS "jetsapi"."file_key_staging" 
ADD COLUMN IF NOT EXISTS "key" integer, 
ADD COLUMN IF NOT EXISTS "client" text, 
ADD COLUMN IF NOT EXISTS "org" text, 
ADD COLUMN IF NOT EXISTS "object_type" text, 
ADD COLUMN IF NOT EXISTS "file_key" text, 
ADD COLUMN IF NOT EXISTS "source_period_key" integer, 
ADD COLUMN IF NOT EXISTS "last_update" timestamp without time zone, 
ADD CONSTRAINT file_key_staging_unique_cstraint UNIQUE (file_key) , 
DROP CONSTRAINT file_key_staging_unique_cstraint  ;

-- Got schema for jetsapi . source_config
CREATE SCHEMA IF NOT EXISTS "jetsapi"
-- Existing Table Constraints for table source_config: [{source_config_unique_cstraint }]
-- existingSchema.TableConstraint: {source_config_unique_cstraint }
-- tableDefinition.TableConstraint: source_config_unique_cstraint1
-- tableDefinition.TableConstraint: source_config_unique_cstraint2
ALTER TABLE IF EXISTS "jetsapi"."source_config" 
ADD COLUMN IF NOT EXISTS "key" integer, 
ADD COLUMN IF NOT EXISTS "object_type" text, 
ADD COLUMN IF NOT EXISTS "client" text, 
ADD COLUMN IF NOT EXISTS "org" text, 
ADD COLUMN IF NOT EXISTS "automated" integer, 
ADD COLUMN IF NOT EXISTS "table_name" text, 
ADD COLUMN IF NOT EXISTS "domain_keys_json" text, 
ADD COLUMN IF NOT EXISTS "user_email" text, 
ADD COLUMN IF NOT EXISTS "last_update" timestamp without time zone, 
ADD CONSTRAINT source_config_unique_cstraint1 UNIQUE (table_name) , 
ADD CONSTRAINT source_config_unique_cstraint2 UNIQUE (client, org, object_type) , 
DROP CONSTRAINT source_config_unique_cstraint  ;

-- Got schema for jetsapi . input_loader_status
CREATE SCHEMA IF NOT EXISTS "jetsapi"
-- Existing Table Constraints for table input_loader_status: [{input_loader_status_unique_cstraint }]
-- existingSchema.TableConstraint: {input_loader_status_unique_cstraint }
-- tableDefinition.TableConstraint: input_loader_status_unique_cstraint
ALTER TABLE IF EXISTS "jetsapi"."input_loader_status" 
ADD COLUMN IF NOT EXISTS "key" integer, 
ADD COLUMN IF NOT EXISTS "object_type" text, 
ADD COLUMN IF NOT EXISTS "table_name" text, 
ADD COLUMN IF NOT EXISTS "client" text, 
ADD COLUMN IF NOT EXISTS "org" text, 
ADD COLUMN IF NOT EXISTS "file_key" text, 
ADD COLUMN IF NOT EXISTS "bad_row_count" integer, 
ADD COLUMN IF NOT EXISTS "load_count" integer, 
ADD COLUMN IF NOT EXISTS "session_id" text, 
ADD COLUMN IF NOT EXISTS "status" text, 
ADD COLUMN IF NOT EXISTS "error_message" text, 
ADD COLUMN IF NOT EXISTS "user_email" text, 
ADD COLUMN IF NOT EXISTS "last_update" timestamp without time zone ;

-- Got schema for jetsapi . input_registry
CREATE SCHEMA IF NOT EXISTS "jetsapi"
-- Existing Table Constraints for table input_registry: [{input_registry_unique_cstraint }]
-- existingSchema.TableConstraint: {input_registry_unique_cstraint }
-- tableDefinition.TableConstraint: input_registry_unique_cstraint
ALTER TABLE IF EXISTS "jetsapi"."input_registry" 
ADD COLUMN IF NOT EXISTS "key" integer, 
ADD COLUMN IF NOT EXISTS "client" text, 
ADD COLUMN IF NOT EXISTS "org" text, 
ADD COLUMN IF NOT EXISTS "object_type" text, 
ADD COLUMN IF NOT EXISTS "file_key" text, 
ADD COLUMN IF NOT EXISTS "source_period_key" integer, 
ADD COLUMN IF NOT EXISTS "table_name" text, 
ADD COLUMN IF NOT EXISTS "source_type" text, 
ADD COLUMN IF NOT EXISTS "session_id" text, 
ADD COLUMN IF NOT EXISTS "user_email" text, 
ADD COLUMN IF NOT EXISTS "last_update" timestamp without time zone ;

-- Got schema for jetsapi . process_input
CREATE SCHEMA IF NOT EXISTS "jetsapi"
-- Existing Table Constraints for table process_input: [{process_input_unique_cstraint }]
-- existingSchema.TableConstraint: {process_input_unique_cstraint }
-- tableDefinition.TableConstraint: process_input_unique_cstraint
ALTER TABLE IF EXISTS "jetsapi"."process_input" 
ADD COLUMN IF NOT EXISTS "key" integer, 
ADD COLUMN IF NOT EXISTS "client" text, 
ADD COLUMN IF NOT EXISTS "org" text, 
ADD COLUMN IF NOT EXISTS "object_type" text, 
ADD COLUMN IF NOT EXISTS "table_name" text, 
ADD COLUMN IF NOT EXISTS "source_type" text, 
ADD COLUMN IF NOT EXISTS "entity_rdf_type" text, 
ADD COLUMN IF NOT EXISTS "key_column" text, 
ADD COLUMN IF NOT EXISTS "status" text, 
ADD COLUMN IF NOT EXISTS "user_email" text, 
ADD COLUMN IF NOT EXISTS "last_update" timestamp without time zone ;

-- Got schema for jetsapi . process_mapping
CREATE SCHEMA IF NOT EXISTS "jetsapi"
-- Existing Table Constraints for table process_mapping: [{process_mapping_unique_cstraint }]
-- existingSchema.TableConstraint: {process_mapping_unique_cstraint }
-- tableDefinition.TableConstraint: process_mapping_unique_cstraint
ALTER TABLE IF EXISTS "jetsapi"."process_mapping" 
ADD COLUMN IF NOT EXISTS "key" integer, 
ADD COLUMN IF NOT EXISTS "table_name" text, 
ADD COLUMN IF NOT EXISTS "input_column" text, 
ADD COLUMN IF NOT EXISTS "data_property" text, 
ADD COLUMN IF NOT EXISTS "function_name" text, 
ADD COLUMN IF NOT EXISTS "argument" text, 
ADD COLUMN IF NOT EXISTS "default_value" text, 
ADD COLUMN IF NOT EXISTS "error_message" text, 
ADD COLUMN IF NOT EXISTS "user_email" text, 
ADD COLUMN IF NOT EXISTS "last_update" timestamp without time zone ;

-- Got schema for jetsapi . process_config
CREATE SCHEMA IF NOT EXISTS "jetsapi"
-- Existing Table Constraints for table process_config: [{process_config_unique_cstraint }]
-- existingSchema.TableConstraint: {process_config_unique_cstraint }
-- tableDefinition.TableConstraint: process_config_unique_cstraint
ALTER TABLE IF EXISTS "jetsapi"."process_config" 
ADD COLUMN IF NOT EXISTS "key" integer, 
ADD COLUMN IF NOT EXISTS "process_name" text, 
ADD COLUMN IF NOT EXISTS "main_rules" text, 
ADD COLUMN IF NOT EXISTS "is_rule_set" integer, 
ADD COLUMN IF NOT EXISTS "output_tables" text ARRAY, 
ADD COLUMN IF NOT EXISTS "description" text, 
ADD COLUMN IF NOT EXISTS "user_email" text, 
ADD COLUMN IF NOT EXISTS "last_update" timestamp without time zone ;

-- Got schema for jetsapi . rule_config
CREATE SCHEMA IF NOT EXISTS "jetsapi"
-- Existing Table Constraints for table rule_config: [{rule_config_unique_cstraint }]
-- existingSchema.TableConstraint: {rule_config_unique_cstraint }
-- tableDefinition.TableConstraint: rule_config_unique_cstraint
ALTER TABLE IF EXISTS "jetsapi"."rule_config" 
ADD COLUMN IF NOT EXISTS "process_config_key" integer, 
ADD COLUMN IF NOT EXISTS "process_name" text, 
ADD COLUMN IF NOT EXISTS "client" text, 
ADD COLUMN IF NOT EXISTS "subject" text, 
ADD COLUMN IF NOT EXISTS "predicate" text, 
ADD COLUMN IF NOT EXISTS "object" text, 
ADD COLUMN IF NOT EXISTS "rdf_type" text ;

-- Got schema for jetsapi . pipeline_config
CREATE SCHEMA IF NOT EXISTS "jetsapi"
-- Existing Table Constraints for table pipeline_config: []
ALTER TABLE IF EXISTS "jetsapi"."pipeline_config" 
ADD COLUMN IF NOT EXISTS "key" integer, 
ADD COLUMN IF NOT EXISTS "process_name" text, 
ADD COLUMN IF NOT EXISTS "client" text, 
ADD COLUMN IF NOT EXISTS "process_config_key" integer, 
ADD COLUMN IF NOT EXISTS "main_process_input_key" integer, 
ADD COLUMN IF NOT EXISTS "merged_process_input_keys" integer ARRAY, 
ADD COLUMN IF NOT EXISTS "main_object_type" text, 
ADD COLUMN IF NOT EXISTS "main_source_type" text, 
ADD COLUMN IF NOT EXISTS "source_period_type" text, 
ADD COLUMN IF NOT EXISTS "lookback_periods" integer, 
ADD COLUMN IF NOT EXISTS "automated" integer, 
ADD COLUMN IF NOT EXISTS "description" text, 
ADD COLUMN IF NOT EXISTS "user_email" text, 
ADD COLUMN IF NOT EXISTS "last_update" timestamp without time zone ;

-- Got schema for jetsapi . pipeline_execution_status
CREATE SCHEMA IF NOT EXISTS "jetsapi"
-- Existing Table Constraints for table pipeline_execution_status: []
ALTER TABLE IF EXISTS "jetsapi"."pipeline_execution_status" 
ADD COLUMN IF NOT EXISTS "key" integer, 
ADD COLUMN IF NOT EXISTS "pipeline_config_key" integer, 
ADD COLUMN IF NOT EXISTS "main_input_registry_key" integer, 
ADD COLUMN IF NOT EXISTS "main_input_file_key" text, 
ADD COLUMN IF NOT EXISTS "merged_input_registry_keys" integer ARRAY, 
ADD COLUMN IF NOT EXISTS "client" text, 
ADD COLUMN IF NOT EXISTS "process_name" text, 
ADD COLUMN IF NOT EXISTS "main_object_type" text, 
ADD COLUMN IF NOT EXISTS "input_session_id" text, 
ADD COLUMN IF NOT EXISTS "session_id" text, 
ADD COLUMN IF NOT EXISTS "status" text, 
ADD COLUMN IF NOT EXISTS "user_email" text, 
ADD COLUMN IF NOT EXISTS "last_update" timestamp without time zone ;

-- Got schema for jetsapi . pipeline_execution_details
CREATE SCHEMA IF NOT EXISTS "jetsapi"
-- Existing Table Constraints for table pipeline_execution_details: []
ALTER TABLE IF EXISTS "jetsapi"."pipeline_execution_details" 
ADD COLUMN IF NOT EXISTS "key" integer, 
ADD COLUMN IF NOT EXISTS "pipeline_config_key" integer, 
ADD COLUMN IF NOT EXISTS "pipeline_execution_status_key" integer, 
ADD COLUMN IF NOT EXISTS "client" text, 
ADD COLUMN IF NOT EXISTS "process_name" text, 
ADD COLUMN IF NOT EXISTS "main_input_session_id" text, 
ADD COLUMN IF NOT EXISTS "session_id" text, 
ADD COLUMN IF NOT EXISTS "shard_id" integer, 
ADD COLUMN IF NOT EXISTS "status" text, 
ADD COLUMN IF NOT EXISTS "error_message" text, 
ADD COLUMN IF NOT EXISTS "input_records_count" integer, 
ADD COLUMN IF NOT EXISTS "rete_sessions_count" integer, 
ADD COLUMN IF NOT EXISTS "output_records_count" integer, 
ADD COLUMN IF NOT EXISTS "user_email" text, 
ADD COLUMN IF NOT EXISTS "last_update" timestamp without time zone ;

-- Got schema for jetsapi . process_errors
CREATE SCHEMA IF NOT EXISTS "jetsapi"
-- Existing Table Constraints for table process_errors: []
ALTER TABLE IF EXISTS "jetsapi"."process_errors" 
ADD COLUMN IF NOT EXISTS "key" integer, 
ADD COLUMN IF NOT EXISTS "session_id" text, 
ADD COLUMN IF NOT EXISTS "grouping_key" text, 
ADD COLUMN IF NOT EXISTS "row_jets_key" text, 
ADD COLUMN IF NOT EXISTS "input_column" text, 
ADD COLUMN IF NOT EXISTS "error_message" text, 
ADD COLUMN IF NOT EXISTS "shard_id" integer, 
ADD COLUMN IF NOT EXISTS "last_update" timestamp without time zone ;

-- Got schema for jetsapi . session_registry
CREATE SCHEMA IF NOT EXISTS "jetsapi"
-- Existing Table Constraints for table session_registry: []
ALTER TABLE IF EXISTS "jetsapi"."session_registry" 
ADD COLUMN IF NOT EXISTS "session_id" text, 
ADD COLUMN IF NOT EXISTS "last_update" timestamp without time zone ;

2023/02/12 22:29:01 Create / Update JetStore Domain Tables
2023/02/12 22:29:01 Opening workspace database...
2023/02/12 22:29:01 Reading table wrs:ClaimFillSummary info...
2023/02/12 22:29:01 Reading table wrs:ClinicalOpportunityTest1 info...
2023/02/12 22:29:01 Reading table wrs:Eligibility info...
2023/02/12 22:29:01 Reading table wrs:PharmacyClaim info...
-- Processing table wrs:Eligibility
-- Existing Table Constraints for table wrs:Eligibility: []
ALTER TABLE IF EXISTS "public"."wrs:Eligibility" 
ADD COLUMN IF NOT EXISTS "jets:key" text, 
ADD COLUMN IF NOT EXISTS "rdf:type" text ARRAY, 
ADD COLUMN IF NOT EXISTS "wrs:client" text, 
ADD COLUMN IF NOT EXISTS "wrs:clientMemberId" text, 
ADD COLUMN IF NOT EXISTS "wrs:coordinationOfBenefits" text, 
ADD COLUMN IF NOT EXISTS "wrs:effectiveDate" date, 
ADD COLUMN IF NOT EXISTS "wrs:employmentStatus" text, 
ADD COLUMN IF NOT EXISTS "wrs:firstName" text, 
ADD COLUMN IF NOT EXISTS "wrs:gender" text, 
ADD COLUMN IF NOT EXISTS "wrs:generatedId" text, 
ADD COLUMN IF NOT EXISTS "wrs:groupNumber" text, 
ADD COLUMN IF NOT EXISTS "wrs:insuranceStatus" text, 
ADD COLUMN IF NOT EXISTS "wrs:lastName" text, 
ADD COLUMN IF NOT EXISTS "wrs:medicareIndicator" integer, 
ADD COLUMN IF NOT EXISTS "wrs:memberAddressLine1" text, 
ADD COLUMN IF NOT EXISTS "wrs:memberAddressLine2" text, 
ADD COLUMN IF NOT EXISTS "wrs:memberAltEmail" text, 
ADD COLUMN IF NOT EXISTS "wrs:memberAltPhone" text, 
ADD COLUMN IF NOT EXISTS "wrs:memberAltPhoneType" text, 
ADD COLUMN IF NOT EXISTS "wrs:memberCity" text, 
ADD COLUMN IF NOT EXISTS "wrs:memberDob" date, 
ADD COLUMN IF NOT EXISTS "wrs:memberEmail" text, 
ADD COLUMN IF NOT EXISTS "wrs:memberNumber" text, 
ADD COLUMN IF NOT EXISTS "wrs:memberPhone" text, 
ADD COLUMN IF NOT EXISTS "wrs:memberPhoneType" text, 
ADD COLUMN IF NOT EXISTS "wrs:memberPronoun" text, 
ADD COLUMN IF NOT EXISTS "wrs:memberSex" text, 
ADD COLUMN IF NOT EXISTS "wrs:memberSsn" text, 
ADD COLUMN IF NOT EXISTS "wrs:memberState" text, 
ADD COLUMN IF NOT EXISTS "wrs:memberZipCode" text, 
ADD COLUMN IF NOT EXISTS "wrs:middleName" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField1Desc" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField1Value" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField2Desc" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField2Value" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField3Desc" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField3Value" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField4Desc" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField4Value" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField5Desc" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField5Value" text, 
ADD COLUMN IF NOT EXISTS "wrs:personCode" text, 
ADD COLUMN IF NOT EXISTS "wrs:planName" text, 
ADD COLUMN IF NOT EXISTS "wrs:planType1" text, 
ADD COLUMN IF NOT EXISTS "wrs:planType2" text, 
ADD COLUMN IF NOT EXISTS "wrs:preferredFirstName" text, 
ADD COLUMN IF NOT EXISTS "wrs:preferredLanguage" text, 
ADD COLUMN IF NOT EXISTS "wrs:primaryDependant" text, 
ADD COLUMN IF NOT EXISTS "wrs:relation" text, 
ADD COLUMN IF NOT EXISTS "wrs:subscriberAddressLine1" text, 
ADD COLUMN IF NOT EXISTS "wrs:subscriberAddressLine2" text, 
ADD COLUMN IF NOT EXISTS "wrs:subscriberCity" text, 
ADD COLUMN IF NOT EXISTS "wrs:subscriberEmail" text, 
ADD COLUMN IF NOT EXISTS "wrs:subscriberId" text, 
ADD COLUMN IF NOT EXISTS "wrs:subscriberPhone" text, 
ADD COLUMN IF NOT EXISTS "wrs:subscriberPhoneType" text, 
ADD COLUMN IF NOT EXISTS "wrs:subscriberSsn" text, 
ADD COLUMN IF NOT EXISTS "wrs:subscriberState" text, 
ADD COLUMN IF NOT EXISTS "wrs:subscriberZipCode" text, 
ADD COLUMN IF NOT EXISTS "wrs:terminationDate" date, 
ADD COLUMN IF NOT EXISTS "session_id" text, 
ADD COLUMN IF NOT EXISTS "last_update" timestamp without time zone ;

-- Processing table wrs:PharmacyClaim
-- Existing Table Constraints for table wrs:PharmacyClaim: []
ALTER TABLE IF EXISTS "public"."wrs:PharmacyClaim" 
ADD COLUMN IF NOT EXISTS "jets:key" text, 
ADD COLUMN IF NOT EXISTS "rdf:type" text ARRAY, 
ADD COLUMN IF NOT EXISTS "wrs:adjudCarrierOperationalId" text, 
ADD COLUMN IF NOT EXISTS "wrs:adjudChainNbr" text, 
ADD COLUMN IF NOT EXISTS "wrs:adjudContractOperationalId" text, 
ADD COLUMN IF NOT EXISTS "wrs:adjudGroupOperationalId" text, 
ADD COLUMN IF NOT EXISTS "wrs:adjudPharmacyAccountNbr" text, 
ADD COLUMN IF NOT EXISTS "wrs:awpUnitCostAmt" double precision, 
ADD COLUMN IF NOT EXISTS "wrs:baseFormularyCopayAmt" double precision, 
ADD COLUMN IF NOT EXISTS "wrs:baseNonFormularyCopayAmt" double precision, 
ADD COLUMN IF NOT EXISTS "wrs:basisCostDeterminationCde" text, 
ADD COLUMN IF NOT EXISTS "wrs:bilCopayAmt" double precision, 
ADD COLUMN IF NOT EXISTS "wrs:bilDispensingFeeAmt" double precision, 
ADD COLUMN IF NOT EXISTS "wrs:bilFinalIngredientCostAmt" double precision, 
ADD COLUMN IF NOT EXISTS "wrs:bilNetCheckAmt" double precision, 
ADD COLUMN IF NOT EXISTS "wrs:bilPatientPayAmt" double precision, 
ADD COLUMN IF NOT EXISTS "wrs:billableDte" date, 
ADD COLUMN IF NOT EXISTS "wrs:billingDawCde" text, 
ADD COLUMN IF NOT EXISTS "wrs:brandGenericCde" text, 
ADD COLUMN IF NOT EXISTS "wrs:calcIngredientCostAmt" double precision, 
ADD COLUMN IF NOT EXISTS "wrs:claimId" text, 
ADD COLUMN IF NOT EXISTS "wrs:claimInvalidInd" text, 
ADD COLUMN IF NOT EXISTS "wrs:claimLineNbr" text, 
ADD COLUMN IF NOT EXISTS "wrs:claimReceivedDte" date, 
ADD COLUMN IF NOT EXISTS "wrs:claimReversedCde" text, 
ADD COLUMN IF NOT EXISTS "wrs:claimStateCde" text, 
ADD COLUMN IF NOT EXISTS "wrs:client" text, 
ADD COLUMN IF NOT EXISTS "wrs:clientGroupId" text, 
ADD COLUMN IF NOT EXISTS "wrs:clientLobId" text, 
ADD COLUMN IF NOT EXISTS "wrs:clientMedicareDTypeCde" text, 
ADD COLUMN IF NOT EXISTS "wrs:clientMemberId" text, 
ADD COLUMN IF NOT EXISTS "wrs:clientMemberRelationshipCde" text, 
ADD COLUMN IF NOT EXISTS "wrs:clientMemberSsn" text, 
ADD COLUMN IF NOT EXISTS "wrs:clientSubscriberId" text, 
ADD COLUMN IF NOT EXISTS "wrs:copayAmt" double precision, 
ADD COLUMN IF NOT EXISTS "wrs:deaNbr" text, 
ADD COLUMN IF NOT EXISTS "wrs:deductAppliedAmt" double precision, 
ADD COLUMN IF NOT EXISTS "wrs:dispensingFeeAmt" double precision, 
ADD COLUMN IF NOT EXISTS "wrs:dispensingProviderNpi" text, 
ADD COLUMN IF NOT EXISTS "wrs:fillCummulativeMedd" double precision, 
ADD COLUMN IF NOT EXISTS "wrs:fillDaysSupplyQty" integer, 
ADD COLUMN IF NOT EXISTS "wrs:fillDispenseDte" date, 
ADD COLUMN IF NOT EXISTS "wrs:fillDrugFormularyInd" text, 
ADD COLUMN IF NOT EXISTS "wrs:fillMedd" double precision, 
ADD COLUMN IF NOT EXISTS "wrs:fillNdc" text, 
ADD COLUMN IF NOT EXISTS "wrs:fillQty" double precision, 
ADD COLUMN IF NOT EXISTS "wrs:fillQtyUnit" text, 
ADD COLUMN IF NOT EXISTS "wrs:generatedId" text, 
ADD COLUMN IF NOT EXISTS "wrs:grossApprovedAmt" double precision, 
ADD COLUMN IF NOT EXISTS "wrs:jurisdictionStateCde" text, 
ADD COLUMN IF NOT EXISTS "wrs:mailRetailCde" text, 
ADD COLUMN IF NOT EXISTS "wrs:medicaidFlag" text, 
ADD COLUMN IF NOT EXISTS "wrs:medicareDLicsCde" text, 
ADD COLUMN IF NOT EXISTS "wrs:netCheckAmt" double precision, 
ADD COLUMN IF NOT EXISTS "wrs:openField1Desc" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField1Value" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField2Desc" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField2Value" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField3Desc" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField3Value" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField4Desc" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField4Value" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField5Desc" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField5Value" text, 
ADD COLUMN IF NOT EXISTS "wrs:otherCovgCde" text, 
ADD COLUMN IF NOT EXISTS "wrs:patientAgeInMonths" integer, 
ADD COLUMN IF NOT EXISTS "wrs:patientFirstName" text, 
ADD COLUMN IF NOT EXISTS "wrs:patientGenderCde" text, 
ADD COLUMN IF NOT EXISTS "wrs:patientLastName" text, 
ADD COLUMN IF NOT EXISTS "wrs:patientLat" text, 
ADD COLUMN IF NOT EXISTS "wrs:patientLon" text, 
ADD COLUMN IF NOT EXISTS "wrs:patientMiddleName" text, 
ADD COLUMN IF NOT EXISTS "wrs:patientPayAmt" double precision, 
ADD COLUMN IF NOT EXISTS "wrs:patientPregnancyCde" text, 
ADD COLUMN IF NOT EXISTS "wrs:patientSalesTaxAmt" double precision, 
ADD COLUMN IF NOT EXISTS "wrs:patientSsn" text, 
ADD COLUMN IF NOT EXISTS "wrs:patientZip5Cde" text, 
ADD COLUMN IF NOT EXISTS "wrs:payCopayAmt" double precision, 
ADD COLUMN IF NOT EXISTS "wrs:payCopayCalculationBasisCde" text, 
ADD COLUMN IF NOT EXISTS "wrs:payDispensingFeeAmt" double precision, 
ADD COLUMN IF NOT EXISTS "wrs:payFinalIngredientCostAmt" double precision, 
ADD COLUMN IF NOT EXISTS "wrs:payIncentiveFeeTotalAmt" double precision, 
ADD COLUMN IF NOT EXISTS "wrs:pharmacyFillNbr" integer, 
ADD COLUMN IF NOT EXISTS "wrs:pharmacyInNetworkInd" text, 
ADD COLUMN IF NOT EXISTS "wrs:pharmacyLat" text, 
ADD COLUMN IF NOT EXISTS "wrs:pharmacyLon" text, 
ADD COLUMN IF NOT EXISTS "wrs:pharmacyNpi" text, 
ADD COLUMN IF NOT EXISTS "wrs:pharmacyPatientId" text, 
ADD COLUMN IF NOT EXISTS "wrs:pharmacyRefillInd" integer, 
ADD COLUMN IF NOT EXISTS "wrs:pharmacyRxNbr" text, 
ADD COLUMN IF NOT EXISTS "wrs:physicianAdminDrugInd" text, 
ADD COLUMN IF NOT EXISTS "wrs:prescriberId" text, 
ADD COLUMN IF NOT EXISTS "wrs:prescriberNpi" text, 
ADD COLUMN IF NOT EXISTS "wrs:priorAuthInd" text, 
ADD COLUMN IF NOT EXISTS "wrs:priorAuthNbr" text, 
ADD COLUMN IF NOT EXISTS "wrs:reversalInd" text, 
ADD COLUMN IF NOT EXISTS "wrs:rxCompoundInd" text, 
ADD COLUMN IF NOT EXISTS "wrs:salesTaxTotalAmt" double precision, 
ADD COLUMN IF NOT EXISTS "wrs:validClaim" text, 
ADD COLUMN IF NOT EXISTS "wrs:wcClaimCde" text, 
ADD COLUMN IF NOT EXISTS "wrs:wcInjuryDte" date, 
ADD COLUMN IF NOT EXISTS "session_id" text, 
ADD COLUMN IF NOT EXISTS "last_update" timestamp without time zone ;

-- Processing table wrs:ClaimFillSummary
-- Existing Table Constraints for table wrs:ClaimFillSummary: []
ALTER TABLE IF EXISTS "public"."wrs:ClaimFillSummary" 
ADD COLUMN IF NOT EXISTS "jets:key" text, 
ADD COLUMN IF NOT EXISTS "rdf:type" text ARRAY, 
ADD COLUMN IF NOT EXISTS "wrs:client" text, 
ADD COLUMN IF NOT EXISTS "wrs:clientMemberId" text, 
ADD COLUMN IF NOT EXISTS "wrs:cummulativeMedd" double precision, 
ADD COLUMN IF NOT EXISTS "wrs:daysSupplyQty" integer, 
ADD COLUMN IF NOT EXISTS "wrs:dispenseDte" date, 
ADD COLUMN IF NOT EXISTS "wrs:drugFormularyInd" text, 
ADD COLUMN IF NOT EXISTS "wrs:eligibilityGeneratedId" text, 
ADD COLUMN IF NOT EXISTS "wrs:generatedId" text, 
ADD COLUMN IF NOT EXISTS "wrs:medd" double precision, 
ADD COLUMN IF NOT EXISTS "wrs:ndc" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField1Desc" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField1Value" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField2Desc" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField2Value" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField3Desc" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField3Value" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField4Desc" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField4Value" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField5Desc" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField5Value" text, 
ADD COLUMN IF NOT EXISTS "wrs:pharmacyClaimGeneratedId" text, 
ADD COLUMN IF NOT EXISTS "wrs:qty" double precision, 
ADD COLUMN IF NOT EXISTS "wrs:qtyUnit" text, 
ADD COLUMN IF NOT EXISTS "session_id" text, 
ADD COLUMN IF NOT EXISTS "last_update" timestamp without time zone ;

-- Processing table wrs:ClinicalOpportunityTest1
-- Existing Table Constraints for table wrs:ClinicalOpportunityTest1: []
ALTER TABLE IF EXISTS "public"."wrs:ClinicalOpportunityTest1" 
ADD COLUMN IF NOT EXISTS "jets:key" text, 
ADD COLUMN IF NOT EXISTS "rdf:type" text ARRAY, 
ADD COLUMN IF NOT EXISTS "wrs:client" text, 
ADD COLUMN IF NOT EXISTS "wrs:clientMemberId" text, 
ADD COLUMN IF NOT EXISTS "wrs:eFirstName" text, 
ADD COLUMN IF NOT EXISTS "wrs:eGender" text, 
ADD COLUMN IF NOT EXISTS "wrs:eGroupNumber" text, 
ADD COLUMN IF NOT EXISTS "wrs:eLastName" text, 
ADD COLUMN IF NOT EXISTS "wrs:eMemberDob" date, 
ADD COLUMN IF NOT EXISTS "wrs:eMemberSsn" text, 
ADD COLUMN IF NOT EXISTS "wrs:ePlanName" text, 
ADD COLUMN IF NOT EXISTS "wrs:ePlanType1" text, 
ADD COLUMN IF NOT EXISTS "wrs:ePlanType2" text, 
ADD COLUMN IF NOT EXISTS "wrs:eligibilityGeneratedId" text, 
ADD COLUMN IF NOT EXISTS "wrs:generatedId" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField1Desc" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField1Value" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField2Desc" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField2Value" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField3Desc" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField3Value" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField4Desc" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField4Value" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField5Desc" text, 
ADD COLUMN IF NOT EXISTS "wrs:openField5Value" text, 
ADD COLUMN IF NOT EXISTS "wrs:pFillNdc" text ARRAY, 
ADD COLUMN IF NOT EXISTS "wrs:pharmacyClaimGeneratedId" text, 
ADD COLUMN IF NOT EXISTS "session_id" text, 
ADD COLUMN IF NOT EXISTS "last_update" timestamp without time zone ;

