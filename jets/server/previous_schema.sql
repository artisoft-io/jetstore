-- michel@pop-os:~/projects/repos/jetstore/jets/update_db$ JETS_DSN_SECRET=rdsSecret9F108BD1-L4s4KvRKqM6h JETS_REGION='us-east-1' JETS_BUCKET=bucket.jetstore.io WORKSPACES_HOME=/home/michel/projects/repos/artisoft-workspaces WORKSPACE=walrus_ws WORKSPACE_DB_PATH=$WORKSPACES_HOME/$WORKSPACE/workspace.db WORKSPACE_LOOKUPS_DB_PATH=$WORKSPACES_HOME/$WORKSPACE/lookup.db JETS_SCHEMA_FILE=~/projects/repos/jetstore/jets/server/previous_jets_schema.json JETSAPI_DB_INIT_PATH=$WORKSPACES_HOME/$WORKSPACE/process_config/workspace_init_db.sql update_db -drop -migrateDb -usingSshTunnel
-- 2023/02/12 22:15:37 Here's what we got:
-- 2023/02/12 22:15:37    -awsDsnSecret: rdsSecret9F108BD1-L4s4KvRKqM6h
-- 2023/02/12 22:15:37    -dbPoolSize: 5
-- 2023/02/12 22:15:37    -usingSshTunnel: true
-- 2023/02/12 22:15:37    -dsn len: 0
-- 2023/02/12 22:15:37    -jetsapiDbInitPath: /home/michel/projects/repos/artisoft-workspaces/walrus_ws/process_config/workspace_init_db.sql
-- 2023/02/12 22:15:37    -workspaceDb: /home/michel/projects/repos/artisoft-workspaces/walrus_ws/workspace.db
-- 2023/02/12 22:15:37    -migrateDb: true
-- 2023/02/12 22:15:37    -initWorkspaceDb: false
-- 2023/02/12 22:15:37 ENV JETSAPI_DB_INIT_PATH: /home/michel/projects/repos/artisoft-workspaces/walrus_ws/process_config/workspace_init_db.sql
-- 2023/02/12 22:15:37 ENV WORKSPACE_DB_PATH: /home/michel/projects/repos/artisoft-workspaces/walrus_ws/workspace.db
-- LOCAL TESTING using ssh tunnel (expecting ssh tunnel open)
-- 2023/02/12 22:15:37 Migrating jetsapi database to latest schema
-- Got schema for jetsapi . jetstore_release
CREATE SCHEMA IF NOT EXISTS "jetsapi"
DROP TABLE IF EXISTS "jetsapi"."jetstore_release"
CREATE TABLE IF NOT EXISTS "jetsapi"."jetstore_release"(
"version" text NOT NULL  PRIMARY KEY ,
"name" text DEFAULT 'JetStore Timestamp Version' NOT NULL );

-- Got schema for jetsapi . users
CREATE SCHEMA IF NOT EXISTS "jetsapi"
Existing Table Constraints for table users: []
ALTER TABLE IF EXISTS "jetsapi"."users" 
ADD COLUMN IF NOT EXISTS "user_email" text, 
ADD COLUMN IF NOT EXISTS "name" text, 
ADD COLUMN IF NOT EXISTS "password" text, 
ADD COLUMN IF NOT EXISTS "is_active" integer, 
ADD COLUMN IF NOT EXISTS "last_update" timestamp without time zone ;

-- Got schema for jetsapi . client_registry
CREATE SCHEMA IF NOT EXISTS "jetsapi"
DROP TABLE IF EXISTS "jetsapi"."client_registry"
CREATE TABLE IF NOT EXISTS "jetsapi"."client_registry"(
"client" text PRIMARY KEY ,
"details" text);

-- Got schema for jetsapi . mapping_function_registry
CREATE SCHEMA IF NOT EXISTS "jetsapi"
DROP TABLE IF EXISTS "jetsapi"."mapping_function_registry"
CREATE TABLE IF NOT EXISTS "jetsapi"."mapping_function_registry"(
"function_name" text PRIMARY KEY ,
"is_argument_required" integer NOT NULL ,
"details" text);

-- Got schema for jetsapi . object_type_registry
CREATE SCHEMA IF NOT EXISTS "jetsapi"
DROP TABLE IF EXISTS "jetsapi"."object_type_registry"
CREATE TABLE IF NOT EXISTS "jetsapi"."object_type_registry"(
"object_type" text PRIMARY KEY ,
"entity_rdf_type" text NOT NULL ,
"details" text);

-- Got schema for jetsapi . domain_keys_registry
CREATE SCHEMA IF NOT EXISTS "jetsapi"
DROP TABLE IF EXISTS "jetsapi"."domain_keys_registry"
CREATE TABLE IF NOT EXISTS "jetsapi"."domain_keys_registry"(
"entity_rdf_type" text PRIMARY KEY ,
"object_types" text ARRAY  NOT NULL ,
"domain_keys_json" text);

-- Got schema for jetsapi . object_type_mapping_details
CREATE SCHEMA IF NOT EXISTS "jetsapi"
DROP TABLE IF EXISTS "jetsapi"."object_type_mapping_details"
CREATE TABLE IF NOT EXISTS "jetsapi"."object_type_mapping_details"(
"object_type" text NOT NULL ,
"data_property" text NOT NULL ,
"is_required" integer NOT NULL ,
"details" text,
CONSTRAINT object_type_mapping_details_unique_cstraint UNIQUE (object_type, data_property));

-- Got schema for jetsapi . file_key_staging
CREATE SCHEMA IF NOT EXISTS "jetsapi"
DROP TABLE IF EXISTS "jetsapi"."file_key_staging"
CREATE TABLE IF NOT EXISTS "jetsapi"."file_key_staging"(
"key"  SERIAL PRIMARY KEY ,
"client" text NOT NULL ,
"object_type" text NOT NULL ,
"file_key" text NOT NULL ,
"last_update" timestamp without time zone DEFAULT now() NOT NULL ,
CONSTRAINT file_key_staging_unique_cstraint UNIQUE (client, object_type, file_key));

-- Got schema for jetsapi . source_config
CREATE SCHEMA IF NOT EXISTS "jetsapi"
DROP TABLE IF EXISTS "jetsapi"."source_config"
CREATE TABLE IF NOT EXISTS "jetsapi"."source_config"(
"key"  SERIAL PRIMARY KEY ,
"object_type" text NOT NULL ,
"client" text NOT NULL ,
"table_name" text NOT NULL ,
"domain_keys_json" text,
"user_email" text NOT NULL ,
"last_update" timestamp without time zone DEFAULT now() NOT NULL ,
CONSTRAINT source_config_unique_cstraint UNIQUE (table_name));
CREATE INDEX source_config_client_idx ON jetsapi.source_config (client ASC) ;
CREATE INDEX source_config_object_type_idx ON jetsapi.source_config (object_type ASC) ;

-- Got schema for jetsapi . input_loader_status
CREATE SCHEMA IF NOT EXISTS "jetsapi"
DROP TABLE IF EXISTS "jetsapi"."input_loader_status"
CREATE TABLE IF NOT EXISTS "jetsapi"."input_loader_status"(
"key"  SERIAL PRIMARY KEY ,
"object_type" text NOT NULL ,
"table_name" text NOT NULL ,
"client" text NOT NULL ,
"file_key" text NOT NULL ,
"bad_row_count" integer,
"load_count" integer,
"session_id" text NOT NULL ,
"status" text NOT NULL ,
"error_message" text,
"user_email" text NOT NULL ,
"last_update" timestamp without time zone DEFAULT now() NOT NULL ,
CONSTRAINT input_loader_status_unique_cstraint UNIQUE (table_name, session_id));
CREATE INDEX input_loader_status_client_idx ON jetsapi.input_loader_status (client ASC) ;
CREATE INDEX input_loader_status_status_idx ON jetsapi.input_loader_status (status ASC) ;
CREATE INDEX input_loader_status_object_type_idx ON jetsapi.input_loader_status (object_type ASC) ;
CREATE INDEX input_loader_status_table_name_idx ON jetsapi.input_loader_status (table_name ASC) ;

-- Got schema for jetsapi . input_registry
CREATE SCHEMA IF NOT EXISTS "jetsapi"
DROP TABLE IF EXISTS "jetsapi"."input_registry"
CREATE TABLE IF NOT EXISTS "jetsapi"."input_registry"(
"key"  SERIAL PRIMARY KEY ,
"client" text NOT NULL ,
"object_type" text NOT NULL ,
"file_key" text,
"table_name" text NOT NULL ,
"source_type" text NOT NULL ,
"session_id" text NOT NULL ,
"user_email" text NOT NULL ,
"last_update" timestamp without time zone DEFAULT now() NOT NULL ,
CONSTRAINT input_registry_unique_cstraint UNIQUE (client, object_type, table_name, session_id));
CREATE INDEX input_registry_client_idx ON jetsapi.input_registry (client ASC) ;
CREATE INDEX input_registry_object_type_idx ON jetsapi.input_registry (object_type ASC) ;
CREATE INDEX input_registry_table_name_idx ON jetsapi.input_registry (table_name ASC) ;

-- Got schema for jetsapi . process_input
CREATE SCHEMA IF NOT EXISTS "jetsapi"
DROP TABLE IF EXISTS "jetsapi"."process_input"
CREATE TABLE IF NOT EXISTS "jetsapi"."process_input"(
"key"  SERIAL PRIMARY KEY ,
"client" text NOT NULL ,
"object_type" text NOT NULL ,
"table_name" text NOT NULL ,
"source_type" text NOT NULL ,
"entity_rdf_type" text NOT NULL ,
"key_column" text,
"status" text DEFAULT 'created' NOT NULL ,
"user_email" text NOT NULL ,
"last_update" timestamp without time zone DEFAULT now() NOT NULL ,
CONSTRAINT process_input_unique_cstraint UNIQUE (client, object_type, table_name));
CREATE INDEX process_input_client_idx ON jetsapi.process_input (client ASC) ;
CREATE INDEX process_input_object_type_idx ON jetsapi.process_input (object_type ASC) ;

-- Got schema for jetsapi . process_mapping
CREATE SCHEMA IF NOT EXISTS "jetsapi"
DROP TABLE IF EXISTS "jetsapi"."process_mapping"
CREATE TABLE IF NOT EXISTS "jetsapi"."process_mapping"(
"key"  SERIAL PRIMARY KEY ,
"table_name" text NOT NULL ,
"input_column" text,
"data_property" text NOT NULL ,
"function_name" text,
"argument" text,
"default_value" text,
"error_message" text,
"user_email" text NOT NULL ,
"last_update" timestamp without time zone DEFAULT now() NOT NULL ,
CONSTRAINT process_mapping_unique_cstraint UNIQUE (table_name, input_column, data_property));
CREATE INDEX process_mapping_table_name_idx ON jetsapi.process_mapping (table_name ASC) ;

-- Got schema for jetsapi . process_config
CREATE SCHEMA IF NOT EXISTS "jetsapi"
DROP TABLE IF EXISTS "jetsapi"."process_config"
CREATE TABLE IF NOT EXISTS "jetsapi"."process_config"(
"key"  SERIAL PRIMARY KEY ,
"process_name" text NOT NULL ,
"main_rules" text NOT NULL ,
"is_rule_set" integer NOT NULL ,
"output_tables" text ARRAY  NOT NULL ,
"description" text,
"user_email" text NOT NULL ,
"last_update" timestamp without time zone DEFAULT now() NOT NULL ,
CONSTRAINT process_config_unique_cstraint UNIQUE (process_name));
CREATE INDEX process_config_process_name_idx ON jetsapi.process_config (process_name ASC) ;

-- Got schema for jetsapi . rule_config
CREATE SCHEMA IF NOT EXISTS "jetsapi"
DROP TABLE IF EXISTS "jetsapi"."rule_config"
CREATE TABLE IF NOT EXISTS "jetsapi"."rule_config"(
"process_config_key" integer NOT NULL ,
"process_name" text NOT NULL ,
"client" text NOT NULL ,
"subject" text NOT NULL ,
"predicate" text NOT NULL ,
"object" text NOT NULL ,
"rdf_type" text NOT NULL ,
CONSTRAINT rule_config_unique_cstraint UNIQUE (process_name, client, subject, predicate, object));
CREATE INDEX rule_config_process_config_key_client_idx ON jetsapi.rule_config (process_config_key ASC, client ASC) ;

-- Got schema for jetsapi . pipeline_config
CREATE SCHEMA IF NOT EXISTS "jetsapi"
DROP TABLE IF EXISTS "jetsapi"."pipeline_config"
CREATE TABLE IF NOT EXISTS "jetsapi"."pipeline_config"(
"key"  SERIAL PRIMARY KEY ,
"process_name" text NOT NULL ,
"client" text NOT NULL ,
"process_config_key" integer NOT NULL ,
"main_process_input_key" integer NOT NULL ,
"merged_process_input_keys" integer ARRAY  DEFAULT '{}' NOT NULL ,
"main_object_type" text NOT NULL ,
"main_source_type" text NOT NULL ,
"automated" integer DEFAULT 0 NOT NULL ,
"description" text,
"user_email" text NOT NULL ,
"last_update" timestamp without time zone DEFAULT now() NOT NULL );
CREATE INDEX pipeline_config_client_idx ON jetsapi.pipeline_config (client ASC) ;
CREATE INDEX pipeline_config_process_name_idx ON jetsapi.pipeline_config (process_name ASC) ;
CREATE INDEX pipeline_config_main_object_type_idx ON jetsapi.pipeline_config (main_object_type ASC) ;

-- Got schema for jetsapi . pipeline_execution_status
CREATE SCHEMA IF NOT EXISTS "jetsapi"
DROP TABLE IF EXISTS "jetsapi"."pipeline_execution_status"
CREATE TABLE IF NOT EXISTS "jetsapi"."pipeline_execution_status"(
"key"  SERIAL PRIMARY KEY ,
"pipeline_config_key" integer NOT NULL ,
"main_input_registry_key" integer,
"main_input_file_key" text,
"merged_input_registry_keys" integer ARRAY  DEFAULT '{}' NOT NULL ,
"client" text NOT NULL ,
"process_name" text NOT NULL ,
"main_object_type" text NOT NULL ,
"input_session_id" text,
"session_id" text NOT NULL ,
"status" text NOT NULL ,
"user_email" text NOT NULL ,
"last_update" timestamp without time zone DEFAULT now() NOT NULL );
CREATE INDEX pipeline_execution_status_last_update_idx ON jetsapi.pipeline_execution_status (last_update DESC) ;

-- Got schema for jetsapi . pipeline_execution_details
CREATE SCHEMA IF NOT EXISTS "jetsapi"
DROP TABLE IF EXISTS "jetsapi"."pipeline_execution_details"
CREATE TABLE IF NOT EXISTS "jetsapi"."pipeline_execution_details"(
"key"  SERIAL PRIMARY KEY ,
"pipeline_config_key" integer NOT NULL ,
"pipeline_execution_status_key" integer NOT NULL ,
"client" text NOT NULL ,
"process_name" text NOT NULL ,
"main_input_session_id" text NOT NULL ,
"session_id" text NOT NULL ,
"shard_id" integer DEFAULT 0 NOT NULL ,
"status" text NOT NULL ,
"error_message" text,
"input_records_count" integer,
"rete_sessions_count" integer,
"output_records_count" integer,
"user_email" text NOT NULL ,
"last_update" timestamp without time zone DEFAULT now() NOT NULL );
CREATE INDEX pipeline_execution_details_pipeline_execution_status_key_idx ON jetsapi.pipeline_execution_details (pipeline_execution_status_key ASC) ;
CREATE INDEX pipeline_execution_details_last_update_idx ON jetsapi.pipeline_execution_details (last_update DESC) ;

-- Got schema for jetsapi . process_errors
CREATE SCHEMA IF NOT EXISTS "jetsapi"
DROP TABLE IF EXISTS "jetsapi"."process_errors"
CREATE TABLE IF NOT EXISTS "jetsapi"."process_errors"(
"key"  SERIAL PRIMARY KEY ,
"session_id" text NOT NULL ,
"grouping_key" text,
"row_jets_key" text,
"input_column" text,
"error_message" text NOT NULL ,
"shard_id" integer DEFAULT 0 NOT NULL ,
"last_update" timestamp without time zone DEFAULT now() NOT NULL );
CREATE INDEX process_errors_session_id_idx ON jetsapi.process_errors (session_id ASC) ;

-- Got schema for jetsapi . session_registry
CREATE SCHEMA IF NOT EXISTS "jetsapi"
DROP TABLE IF EXISTS "jetsapi"."session_registry"
CREATE TABLE IF NOT EXISTS "jetsapi"."session_registry"(
"session_id" text NOT NULL  PRIMARY KEY ,
"last_update" timestamp without time zone DEFAULT now() NOT NULL );

2023/02/12 22:15:40 Create / Update JetStore Domain Tables
2023/02/12 22:15:40 Opening workspace database...
2023/02/12 22:15:40 Reading table wrs:ClaimFillSummary info...
2023/02/12 22:15:40 Reading table wrs:ClinicalOpportunityTest1 info...
2023/02/12 22:15:40 Reading table wrs:Eligibility info...
2023/02/12 22:15:40 Reading table wrs:PharmacyClaim info...
-- Processing table wrs:ClaimFillSummary
DROP TABLE IF EXISTS "public"."wrs:ClaimFillSummary"
CREATE TABLE IF NOT EXISTS "public"."wrs:ClaimFillSummary"(
"jets:key" text NOT NULL ,
"rdf:type" text ARRAY ,
"wrs:client" text,
"wrs:clientMemberId" text,
"wrs:cummulativeMedd" double precision,
"wrs:daysSupplyQty" integer,
"wrs:dispenseDte" date,
"wrs:drugFormularyInd" text,
"wrs:eligibilityGeneratedId" text,
"wrs:generatedId" text,
"wrs:medd" double precision,
"wrs:ndc" text,
"wrs:openField1Desc" text,
"wrs:openField1Value" text,
"wrs:openField2Desc" text,
"wrs:openField2Value" text,
"wrs:openField3Desc" text,
"wrs:openField3Value" text,
"wrs:openField4Desc" text,
"wrs:openField4Value" text,
"wrs:openField5Desc" text,
"wrs:openField5Value" text,
"wrs:pharmacyClaimGeneratedId" text,
"wrs:qty" double precision,
"wrs:qtyUnit" text,
"session_id" text NOT NULL ,
"last_update" timestamp without time zone DEFAULT now() NOT NULL );

-- Processing table wrs:ClinicalOpportunityTest1
DROP TABLE IF EXISTS "public"."wrs:ClinicalOpportunityTest1"
CREATE TABLE IF NOT EXISTS "public"."wrs:ClinicalOpportunityTest1"(
"jets:key" text NOT NULL ,
"rdf:type" text ARRAY ,
"wrs:client" text,
"wrs:clientMemberId" text,
"wrs:eFirstName" text,
"wrs:eGender" text,
"wrs:eGroupNumber" text,
"wrs:eLastName" text,
"wrs:eMemberDob" date,
"wrs:eMemberSsn" text,
"wrs:ePlanName" text,
"wrs:ePlanType1" text,
"wrs:ePlanType2" text,
"wrs:eligibilityGeneratedId" text,
"wrs:generatedId" text,
"wrs:openField1Desc" text,
"wrs:openField1Value" text,
"wrs:openField2Desc" text,
"wrs:openField2Value" text,
"wrs:openField3Desc" text,
"wrs:openField3Value" text,
"wrs:openField4Desc" text,
"wrs:openField4Value" text,
"wrs:openField5Desc" text,
"wrs:openField5Value" text,
"wrs:pFillNdc" text ARRAY ,
"wrs:pharmacyClaimGeneratedId" text,
"session_id" text NOT NULL ,
"last_update" timestamp without time zone DEFAULT now() NOT NULL );

-- Processing table wrs:Eligibility
DROP TABLE IF EXISTS "public"."wrs:Eligibility"
CREATE TABLE IF NOT EXISTS "public"."wrs:Eligibility"(
"jets:key" text NOT NULL ,
"rdf:type" text ARRAY ,
"wrs:client" text,
"wrs:clientMemberId" text,
"wrs:coordinationOfBenefits" text,
"wrs:effectiveDate" date,
"wrs:employmentStatus" text,
"wrs:firstName" text,
"wrs:gender" text,
"wrs:generatedId" text,
"wrs:groupNumber" text,
"wrs:insuranceStatus" text,
"wrs:lastName" text,
"wrs:medicareIndicator" integer,
"wrs:memberAddressLine1" text,
"wrs:memberAddressLine2" text,
"wrs:memberAltEmail" text,
"wrs:memberAltPhone" text,
"wrs:memberAltPhoneType" text,
"wrs:memberCity" text,
"wrs:memberDob" date,
"wrs:memberEmail" text,
"wrs:memberNumber" text,
"wrs:memberPhone" text,
"wrs:memberPhoneType" text,
"wrs:memberPronoun" text,
"wrs:memberSex" text,
"wrs:memberSsn" text,
"wrs:memberState" text,
"wrs:memberZipCode" text,
"wrs:middleName" text,
"wrs:openField1Desc" text,
"wrs:openField1Value" text,
"wrs:openField2Desc" text,
"wrs:openField2Value" text,
"wrs:openField3Desc" text,
"wrs:openField3Value" text,
"wrs:openField4Desc" text,
"wrs:openField4Value" text,
"wrs:openField5Desc" text,
"wrs:openField5Value" text,
"wrs:personCode" text,
"wrs:planName" text,
"wrs:planType1" text,
"wrs:planType2" text,
"wrs:preferredFirstName" text,
"wrs:preferredLanguage" text,
"wrs:primaryDependant" text,
"wrs:relation" text,
"wrs:subscriberAddressLine1" text,
"wrs:subscriberAddressLine2" text,
"wrs:subscriberCity" text,
"wrs:subscriberEmail" text,
"wrs:subscriberId" text,
"wrs:subscriberPhone" text,
"wrs:subscriberPhoneType" text,
"wrs:subscriberSsn" text,
"wrs:subscriberState" text,
"wrs:subscriberZipCode" text,
"wrs:terminationDate" date,
"session_id" text NOT NULL ,
"last_update" timestamp without time zone DEFAULT now() NOT NULL );

-- Processing table wrs:PharmacyClaim
DROP TABLE IF EXISTS "public"."wrs:PharmacyClaim"
CREATE TABLE IF NOT EXISTS "public"."wrs:PharmacyClaim"(
"jets:key" text NOT NULL ,
"rdf:type" text ARRAY ,
"wrs:adjudCarrierOperationalId" text,
"wrs:adjudChainNbr" text,
"wrs:adjudContractOperationalId" text,
"wrs:adjudGroupOperationalId" text,
"wrs:adjudPharmacyAccountNbr" text,
"wrs:awpUnitCostAmt" double precision,
"wrs:baseFormularyCopayAmt" double precision,
"wrs:baseNonFormularyCopayAmt" double precision,
"wrs:basisCostDeterminationCde" text,
"wrs:bilCopayAmt" double precision,
"wrs:bilDispensingFeeAmt" double precision,
"wrs:bilFinalIngredientCostAmt" double precision,
"wrs:bilNetCheckAmt" double precision,
"wrs:bilPatientPayAmt" double precision,
"wrs:billableDte" date,
"wrs:billingDawCde" text,
"wrs:brandGenericCde" text,
"wrs:calcIngredientCostAmt" double precision,
"wrs:claimId" text,
"wrs:claimInvalidInd" text,
"wrs:claimLineNbr" text,
"wrs:claimReceivedDte" date,
"wrs:claimReversedCde" text,
"wrs:claimStateCde" text,
"wrs:client" text,
"wrs:clientGroupId" text,
"wrs:clientLobId" text,
"wrs:clientMedicareDTypeCde" text,
"wrs:clientMemberId" text,
"wrs:clientMemberRelationshipCde" text,
"wrs:clientMemberSsn" text,
"wrs:clientSubscriberId" text,
"wrs:copayAmt" double precision,
"wrs:deaNbr" text,
"wrs:deductAppliedAmt" double precision,
"wrs:dispensingFeeAmt" double precision,
"wrs:dispensingProviderNpi" text,
"wrs:fillCummulativeMedd" double precision,
"wrs:fillDaysSupplyQty" integer,
"wrs:fillDispenseDte" date,
"wrs:fillDrugFormularyInd" text,
"wrs:fillMedd" double precision,
"wrs:fillNdc" text,
"wrs:fillQty" double precision,
"wrs:fillQtyUnit" text,
"wrs:generatedId" text,
"wrs:grossApprovedAmt" double precision,
"wrs:jurisdictionStateCde" text,
"wrs:mailRetailCde" text,
"wrs:medicaidFlag" text,
"wrs:medicareDLicsCde" text,
"wrs:netCheckAmt" double precision,
"wrs:openField1Desc" text,
"wrs:openField1Value" text,
"wrs:openField2Desc" text,
"wrs:openField2Value" text,
"wrs:openField3Desc" text,
"wrs:openField3Value" text,
"wrs:openField4Desc" text,
"wrs:openField4Value" text,
"wrs:openField5Desc" text,
"wrs:openField5Value" text,
"wrs:otherCovgCde" text,
"wrs:patientAgeInMonths" integer,
"wrs:patientFirstName" text,
"wrs:patientGenderCde" text,
"wrs:patientLastName" text,
"wrs:patientLat" text,
"wrs:patientLon" text,
"wrs:patientMiddleName" text,
"wrs:patientPayAmt" double precision,
"wrs:patientPregnancyCde" text,
"wrs:patientSalesTaxAmt" double precision,
"wrs:patientSsn" text,
"wrs:patientZip5Cde" text,
"wrs:payCopayAmt" double precision,
"wrs:payCopayCalculationBasisCde" text,
"wrs:payDispensingFeeAmt" double precision,
"wrs:payFinalIngredientCostAmt" double precision,
"wrs:payIncentiveFeeTotalAmt" double precision,
"wrs:pharmacyFillNbr" integer,
"wrs:pharmacyInNetworkInd" text,
"wrs:pharmacyLat" text,
"wrs:pharmacyLon" text,
"wrs:pharmacyNpi" text,
"wrs:pharmacyPatientId" text,
"wrs:pharmacyRefillInd" integer,
"wrs:pharmacyRxNbr" text,
"wrs:physicianAdminDrugInd" text,
"wrs:prescriberId" text,
"wrs:prescriberNpi" text,
"wrs:priorAuthInd" text,
"wrs:priorAuthNbr" text,
"wrs:reversalInd" text,
"wrs:rxCompoundInd" text,
"wrs:salesTaxTotalAmt" double precision,
"wrs:validClaim" text,
"wrs:wcClaimCde" text,
"wrs:wcInjuryDte" date,
"session_id" text NOT NULL ,
"last_update" timestamp without time zone DEFAULT now() NOT NULL );
