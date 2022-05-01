DROP TABLE IF EXISTS rchc__pharmacyclaim;
CREATE TABLE IF NOT EXISTS rchc__pharmacyclaim (
    rchc__claim_amount double precision  ,
    rchc__claim_number text  ,
    rchc__claim_type text  ,
    rchc__days_of_supplies integer  ,
    rchc__long_code bigint  ,
    rchc__long_code2 bigint ARRAY  ,
    rchc__dispense_date date  ,
    rchc__drug_ndc text  ,
    rchc__drug_qty double precision  ,
    rchc__payor_member_number text  ,
    rchc__service_date date  ,
    rdf__type text ARRAY ,
    rdv_core__domainkey text  ,
    rdv_core__key text  ,
    rdv_core__persisted_data_type text  ,
    rdv_core__sessionid text  ,
    shard_id integer DEFAULT 0 NOT NULL,
    last_update timestamp without time zone DEFAULT now() NOT NULL
);

CREATE INDEX IF NOT EXISTS rchc__pharmacyclaim_primary_idx ON rchc__pharmacyclaim (rdv_core__key, rdv_core__sessionid, last_update DESC);
CREATE INDEX IF NOT EXISTS rchc__pharmacyclaim_shard_id_idx ON rchc__pharmacyclaim (shard_id);


ALTER TABLE IF EXISTS "hc__claim" ADD COLUMN IF NOT EXISTS "lkrow" TEXT ARRAY ;
