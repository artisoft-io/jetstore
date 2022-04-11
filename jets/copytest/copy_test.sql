DROP TABLE IF EXISTS hc__pharmacyclaim;
CREATE TABLE IF NOT EXISTS hc__pharmacyclaim (
    hc__claim_amount double precision  ,
    hc__claim_number text  ,
    hc__claim_type text  ,
    hc__days_of_supplies integer  ,
    hc__dispense_date date  ,
    hc__drug_ndc text  ,
    hc__drug_qty double precision  ,
    hc__payor_member_number text  ,
    hc__service_date date  ,
    rdf__type text ARRAY ,
    jets__key text  ,
    jets__persisted_data_type text  ,
    jets__sessionid text  ,
    shard_id integer DEFAULT 0 NOT NULL,
    last_update timestamp without time zone DEFAULT now() NOT NULL
);
CREATE INDEX IF NOT EXISTS hc__pharmacyclaim_primary_idx ON hc__pharmacyclaim (jets__key, jets__sessionid, last_update DESC);
CREATE INDEX IF NOT EXISTS hc__pharmacyclaim_shard_id_idx ON hc__pharmacyclaim (shard_id);

INSERT INTO hc__pharmacyclaim (
  hc__claim_amount,
  hc__claim_number,
  hc__claim_type,
  hc__days_of_supplies,
  hc__dispense_date,
  hc__drug_ndc,
  hc__drug_qty,
  hc__payor_member_number,
  hc__service_date,
  rdf__type,
  jets__key,
  jets__persisted_data_type,
  jets__sessionid) 
  VALUES 
    (121.12,'12345678901','phar|macy',30,'04-08-2022','NDC1',60,'PAYOR01','04-08-2022','{"jets:Entity","hc:PharmacyClaim"}','KEY01','hc:PharmacyClaim','SESSION01'),
    (281.22,'12345678902','pharmacy',45,'03-18-2022','NDC2',90,'PAYOR02','03-18-2022','{"jets:Entity","hc:PharmacyClaim"}','KEY02','hc:PharmacyClaim','SESSION01')
;

COPY hc__pharmacyclaim  TO '/work/hc__pharmacyclaim.copy' WITH (DELIMITER '|');
