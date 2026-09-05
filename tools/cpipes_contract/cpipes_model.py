"""The cpipes contract - THE SOURCE OF TRUTH for the contract claims (B.10).

Since the B.10 flip, this Pydantic v2 model is where the contract is edited:
which fields exist per operator token, which are required, their value ranges,
and their descriptions. The matrix CSVs remain the review artifact and the
audit/Go-binding record, regenerated FROM this file:

    python -m cpipes_contract reflect          # sync the matrix claim columns
    python -m cpipes_contract reflect --check  # the divergence guard (CI)

The `generate` command is the bootstrap projection that first produced this
file from the reviewed matrix (B.9) - running it again OVERWRITES edits made
here. Engine-applied defaults are noted in the descriptions and deliberately
NOT materialised as Pydantic defaults, so a dumped document stays minimal and
the wire format unchanged.
"""

from __future__ import annotations

from typing import Annotated, Any, Literal, Union

from pydantic import BaseModel, BeforeValidator, ConfigDict, Field


def _tag_default(key: str, default: str):
    """Inject the engine's default discriminator value when the document
    omits it, so the discriminated union accepts untyped instances the way
    the engine does."""

    def inject(value):
        if isinstance(value, dict) and key not in value:
            return {**value, key: default}
        return value

    return inject


class _Base(BaseModel):
    model_config = ConfigDict(extra="forbid")


class AnalyzeSpec(_Base):
    """Configuration of the analyze transformation operator."""
    column_name_token: ColumnNameTokenNode | None = Field(default=None, description="ColumnNameToken is used to classify columns based on their name.")
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    distinct_values_when_less_than_count: int | None = Field(default=None, description="DistinctValuesWhenLessThanCount is the threshold to list distinct values. Engine default: 20 (builder).")
    entity_hints: list[EntityHint] | None = Field(default=None, description="EntityHints provide hints for entity recognition.")
    function_tokens: list[FunctionTokenNode] | None = Field(default=None, description="FunctionTokens specify functions to identify classification tokens.")
    keyword_tokens: list[KeywordTokenNode] | None = Field(default=None, description="KeywordTokens specify keywords to identify classification tokens.")
    lookup_tokens: list[LookupTokenNode] | None = Field(default=None, description="LookupTokens specify lookup tables to identify classification tokens.")
    pad_short_rows_with_nulls: bool | None = Field(default=None, description="PadShortRowsWithNulls indicates to pad short rows with nulls to match row length.")
    regex_tokens: list[RegexNode] | None = Field(default=None, description="RegexTokens specify regex patterns to identify classification tokens.")
    schema_provider: str | None = Field(default=None, description="SchemaProvider is used for external configuration, such as date format")
    scrub_chars: str | None = Field(default=None, description="ScrubChars is the list of characters to scrub from the input values.")


class AnonymizeSpecBase(_Base):
    """AnonymizeSpec: Configuration for the anonymize transformation operator: lookup information indicating how to anonymize, etc."""
    adjust_field_width_on_fixed_width_file: bool | None = Field(default=None, description="Adjust field widths on fixed-width files.")
    anonymize_type: str = Field(description="AnonymizeType is column name in lookup table that specifies how to anonymize (value: date, text).")
    anonymized_columns_output_file: ColumnFileSpec | None = Field(default=None, description="Where the list of anonymized columns is written.")
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    date_formats_column: str | None = Field(default=None, description="DateFormatsColumn is column name of the lookup table having the list of date format (optional).")
    default_invalid_date: str | None = Field(default=None, description="DefaultInvalidDate is a placeholder to use as the anonymized date when the input date (the date to anonymize) is not valid. If unspecified, the input value is used unchanged as the output value. DefaultInvalidDate must be a valid date in the format YYYY/MM/DD or MM/DD/YYYY so it can be parsed using JetStore default date parser. The date will be formatted according to KeyDateLayout. OutputDateLayout defaults to InputDateLayout. KeyDateLayout defaults to OutputDateLayout.")
    input_date_layout: str | None = Field(default=None, description="InputDateLayout is the format for parsing the input date (incoming data) when specified.")
    key_prefix: str = Field(description="KeyPrefix is column name of lookup table to use as prefix of the anonymized value or key mapping for de-identification lookup table.")
    lookup_name: str = Field(description="LookupName is name of lookup table containing the file metadata from analyze operator.")
    output_date_layout: str | None = Field(default=None, description="OutputDateLayout is the format to use for anonymized date, will be set at 1st of the month of the original date (anonymization) or to XXX when de-identification. Engine default: 2006/01/02 (builder).")
    schema_provider: str | None = Field(default=None, description="SchemaProvider is used to: - get the DateLayout / KeyDateLayout if not specified here. - get CapDobYears / SetDodToJan1 for date anonymization. If date format is not specified, the default format for both OutputDateFormat and KeyDateFormat is \"2006/01/02\", ie. yyyy/MM/dd and the rdf.ParseDate() is used to parse the input date.")


class AnonymizeSpecAnonymization(AnonymizeSpecBase):
    """Configuration for the anonymize transformation operator: lookup information indicating how to anonymize, etc."""
    mode: Literal["anonymization"] = Field(default="anonymization", description="Mode: Specify mode of action: de-identification, anonymization (default) - de-identification: mask the data (not reversible); - anonymization: replace the data with hashed value (reversible using crosswalk file). Engine default: anonymization (builder).")
    data_classification: str = Field(description="The column of the metadata lookup table giving each input column's data classification. The classification decides how the column is treated (e.g. 'date' columns can be capped to Jan 1) and, in de-identification mode, selects the deid lookup (deid_lookups) or deid function (deid_functions) to apply.")
    key_date_layout: str | None = Field(default=None, description="KeyDateLayout is the format to use in the key mapping file (crosswalk file) for anonymization.")
    keys_output_channel: OutputChannelConfig = Field(description="Channel receiving the key mapping (crosswalk) records.")
    omit_prefix_on_fixed_width_file: bool | None = Field(default=None, description="Omit the key prefix on fixed-width files.")


class AnonymizeSpecDeIdentification(AnonymizeSpecBase):
    """Configuration for the de-identification transformation operator: lookup for replacement values, etc."""
    mode: Literal["de-identification"] = Field(description="Mode: Specify mode of action: de-identification, anonymization (default) - de-identification: mask the data (not reversible); - anonymization: replace the data with hashed value (reversible using crosswalk file).")
    data_classification: str | None = Field(default=None, description="The column of the metadata lookup table giving each input column's data classification. The classification decides how the column is treated (e.g. 'date' columns can be capped to Jan 1) and, in de-identification mode, selects the deid lookup (deid_lookups) or deid function (deid_functions) to apply.")
    deid_functions: dict[str, str] = Field(description="DeidFunctions is map of KeyPrefix value to function name for de-identification.")
    deid_lookups: dict[str, str] = Field(description="DeidLookups is map of KeyPrefix value to lookup table name for substitution values for de-identification.")


class BadRowsSpec(_Base):
    """Specify step id folder name in JetStore s3 stage location to save bad rows from input channel."""
    bad_rows_step_id: str | None = Field(default=None, description="BadRowsStepId: step id in stage location to output bad rows The input row is considered a bad row when any of WhenCriteria applies then the row is sent to bad row channel and remove from the input rows. WhenCriteria []BadRowsCriteria `json:\"when_criteria,omitempty\"`")
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")


class BlankFieldMarkersSpec(_Base):
    """Configure marker text that indicate the value is considered as null or blank"""
    case_sensitive: bool | None = Field(default=None, description="Match the markers case-sensitively.")
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    markers: list[str] | None = Field(default=None, description="The marker texts.")


class CaseEnvExpression(_Base):
    """Case expression as when-then using env variables"""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    then: list[ExpressionNode] | None = Field(default=None, description="The env var assignments applied when the condition holds.")
    when: ExpressionNode = Field(description="The condition of this case leg.")


class CaseExpression(_Base):
    """Defines the when-then building block of a case expression."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    then: list[TransformationColumnSpec] | None = Field(default=None, description="The column transformations applied when the condition holds.")
    when: ExpressionNode = Field(description="The condition of this case leg.")


class ChannelSpec(_Base):
    """Shared configuration of the row structure of the input and output channels."""
    class_name: str | None = Field(default=None, description="ClassName is used to get the columns from the local workspace, and get domain key from registry, and is optional. Env variables (from mainInputSchemaProvider.Env) can be used in the class_name, e.g., hc:${ENTITY}.")
    column_encodings: list[ColumnEncodingSpec] | None = Field(default=None, description="Special encodings (json, toon) for selected columns.")
    columns: list[str] | None = Field(default=None, description="The channel's columns; added to the domain class's when class_name is set. Required when: absent(same_columns_as_input).")
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    direct_properties_only: bool | None = Field(default=None, description="Take only the class's own data properties, excluding the parents'.")
    domain_keys: dict[str, str | list[str]] | None = Field(default=None, description="DomainKeys provide the ability to configure the domain keys in the cpipes config document.")
    domain_keys_spec: DomainKeysSpec | None = Field(default=None, description="DomainKeysInfo is obtained from the domain_keys_registry table or derived from DomainKeys - the latter takes precedence when both are available. columnsMap is added in StartComputePipes")
    has_dynamic_columns: bool | None = Field(default=None, description="The channel's columns are extended at run time.")
    name: str = Field(description="The channel name referenced by input and output channels.")
    same_columns_as_input: bool | None = Field(default=None, description="Use the same columns as the input channel.")


class ClusterShardingInfo(_Base):
    """Pipeline configuration for worker nodes - internal to JetStore"""
    max_nbr_partitions: int | None = Field(default=None, description="Cap on the number of partitions.")
    multi_step_sharding: int | None = Field(default=None, description="Multi-step sharding indicator; calculated at sharding.")
    nbr_partitions: int | None = Field(default=None, description="Nbr of partitions; calculated at sharding.")
    total_file_size: int | None = Field(default=None, description="Total input size in bytes; calculated at sharding.")


class ClusterShardingSpec(_Base):
    """Specify a tiered configuration based on file size"""
    applies_to_format: str | None = Field(default=None, description="File format this tier applies to; empty means all formats.")
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    max_concurrency: int | None = Field(default=None, description="Max concurrent nodes: ECS cluster tasks, or lambdas otherwise.")
    max_nbr_partitions: int | None = Field(default=None, description="Cap on partitions for this tier; defaults to the cluster-level value.")
    multi_step_sharding_thresholds: int | None = Field(default=None, description="Partition count triggering multi-step sharding.")
    s3_worker_pool_size: int | None = Field(default=None, description="Worker pool size for s3 operations (used by reducing01).")
    shard_max_size_by: float | None = Field(default=None, description="Max shard size in bytes; for testing only.")
    shard_max_size_mb: float | None = Field(default=None, description="Max shard size in MB; must be set for sharding to take place.")
    shard_size_by: float | None = Field(default=None, description="Shard size in bytes; for testing only.")
    shard_size_mb: float | None = Field(default=None, description="Target shard size in MB; must be set for sharding to take place.")
    when_total_size_ge_mb: int | None = Field(default=None, description="WhenTotalSizeGe: Specify the condition for this to be applied in MB (read as \"when total size greater than or equal to\"). When using ecs tasks, MaxConcurrency applies to ECS cluster, otherwise MaxConcurrency is the number of concurrent lambda functions executing.")


class ClusterSpec(_Base):
    """Configuration for the execution of a pipeline (shard size, nbr worker nodes, etc.)"""
    cluster_sharding_tiers: list[ClusterShardingSpec] | None = Field(default=None, description="Tiered sizing configuration by total input size.")
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    default_max_concurrency: int | None = Field(default=None, description="[DefaultMaxConcurrency] is to override the env var TASK_MAX_CONCURRENCY [nbrPartitions] is specified at ClusterShardingSpec level otherwise at the ClusterSpec level. [nbrPartitions] is determined by the nbr of sharding nodes, capped by MaxNbrPartitions.")
    default_shard_max_size_by: float | None = Field(default=None, description="[DefaultShardMaxSizeBy] is the default value (in bytes) when not specified at ClusterShardingSpec level.")
    default_shard_max_size_mb: float | None = Field(default=None, description="[DefaultShardMaxSizeMb] is the default value (in MB) when not specified at ClusterShardingSpec level.")
    default_shard_size_by: float | None = Field(default=None, description="[DefaultShardSizeBy] is the default value (in bytes) when not specified at ClusterShardingSpec level.")
    default_shard_size_mb: float | None = Field(default=None, description="[DefaultShardSizeMb] is the default value (in MB) when not specified at ClusterShardingSpec level.")
    is_debug_mode: bool | None = Field(default=None, description="Log trace info.")
    kill_switch_min: int | None = Field(default=None, description="Abort the pipeline after this many minutes.")
    max_nbr_partitions: int | None = Field(default=None, description="Cap on the number of partitions.")
    multi_step_sharding_thresholds: int | None = Field(default=None, description="[MultiStepShardingThresholds] is the number of partitions to trigger the use of multi step sharding. When [MultiStepShardingThresholds] > 0 then [nbrPartitions] is sqrt(nbr of sharding nodes).")
    s3_worker_pool_size: int | None = Field(default=None, description="Worker pool size for s3 operations.")
    shard_offset: int | None = Field(default=None, description="Enables splitting large input files into byte-range shards and gives the boundary-alignment window in bytes: a reader whose range starts mid-file scans the first shard_offset bytes of its range for the last end-of-line and starts reading after it. 0 disables file splitting.")


class ClusteringSpec(_Base):
    """Configuration for the clustering transformation operator."""
    cluster_data_subclassification: list[str] | None = Field(default=None, description="data_classification values propagated as data_subclassification across a cluster.")
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    correlation_output_channel: OutputChannelConfig = Field(description="Intermediate channel between the pool manager and the workers.")
    is_debug: bool | None = Field(default=None, description="Forward the correlation results to s3.")
    max_input_count: int | None = Field(default=None, description="Cap on the input records.")
    min_column1_non_null_count: int = Field(description="MinColumn1NonNilCount is min nbr of column1 distinct values observed")
    min_column2_non_null_count: int = Field(description="MinColumn2NonNilCount is min nbr of non nil values of column2 for a worker to report the correlation. ClusterDataSubclassification contains data_classification values, when found in a cluster all columns member of the cluster get that value as data_subclassification.")
    solo_data_subclassification: list[str] | None = Field(default=None, description="The data_classification values eligible as data_subclassification of a solo cluster: when a cluster ends up with a single member column whose classification is in this list, that value is reported as the column's data_subclassification.")
    target_columns_lookup: TargetColumnsLookupSpec = Field(description="Lookup identifying the column sets to correlate.")
    transitive_data_classification: list[str] | None = Field(default=None, description="The data_classification values treated as transitive when forming clusters: a column bearing one of these classifications can merge the two clusters it correlates with into one, letting correlations be followed through it.")


class ColumnEncodingSpec(_Base):
    """JetRules configuration for encoding entity from rule session into an output column"""
    column: str = Field(description="Column is the column name to which the special encoding applies, this is required.")
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    entity_encoding: Literal["json", "toon"] | None = Field(default=None, description="EntityEncoding is used to specify the encoding of the column: range values: json, toon (default is json).")
    exclude_properties: list[str] | None = Field(default=None, description="ExcludeProperties is used to specify the properties to exclude from the output, e.g., jets:key, rdf:type, etc. This is used to exclude properties from the json or toon output.")
    remove_model_prefixes: bool | None = Field(default=None, description="RemoveModelPrefixes is used to remove the model prefixes from the columns, e.g., jets: or rdf: on the output (any prefix up to the character ':').")


class ColumnFileSpec(_Base):
    """Where the anonymize operator writes its columns file: bucket (empty for JetStore one), output location, schema provider and delimiter."""
    bucket: str | None = Field(default=None, description="Bucket, or empty for JetStore one. Engine default: jetstore_bucket (builder).")
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    delimiter: int = Field(description="Rune delimiter to use for the output file.")
    output_location: str = Field(description="Custom file key.")
    schema_provider: str | None = Field(default=None, description="Key of the schema provider associated with this file.")


class ColumnNameLookupNode(_Base):
    """Mapping of column classification name to column name"""
    column_name_fragments: list[str] | None = Field(default=None, description="ColumnNameFragments: list of column name fragments, if a column name contains any of the fragments, it maps to the classification token. ColumnNames takes precedence over ColumnPos. Both can be empty if ColumnNameFragments is used.")
    column_names: list[str] | None = Field(default=None, description="ColumnNames: list of column names that map to the classification token")
    column_pos: list[int] | None = Field(default=None, description="ColumnPos: list of column positions (0 based) that map to the classification token")
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    name: str = Field(description="Name: classification token name")


class ColumnNameTokenNode(_Base):
    """Configuration for classifying column by name via a lookup."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    lookup: list[ColumnNameLookupNode] | None = Field(default=None, description="Lookup: list of ColumnNameLookupNode to match the column names to the classification token.")
    name: str = Field(description="Name: correspond to the name of the output column where the classification token is stored")


class ComputePipesCommonArgs(_Base):
    """Pipeline configuration for worker nodes - internal to JetStore"""
    client: str | None = Field(default=None, description="The pipeline execution client, taken from the main input's schema provider. May be a stand-in name; the actual client of the data is then given at run time by the schema providers registered in input_registry.")
    cpipes_mode: str | None = Field(default=None, description="The execution phase of this run. Range: sharding, reducing. Set by the start lambdas; drives how the file readers and the coordinator behave.")
    domain_keys_by_class: dict[str, DomainKeysSpec] | None = Field(default=None, description="The domain-keys spec of each domain class in play, loaded from table domain_keys_registry. At DAG construction it supplies the DomainKeySpec of channels declared with a class_name that carry no explicit domain-keys info.")
    file_key: str | None = Field(default=None, description="The S3 key of the main input file of this pipeline execution.")
    input_session_id: str | None = Field(default=None, description="The session id of the prior pipeline execution that produced this run's input data.")
    merge_files: bool | None = Field(default=None, description="True when the run is at the merge_files stage: a single node concatenates the part files written by the previous step into a single output file.")
    object_type: str | None = Field(default=None, description="The object type of the main input, taken from the main input's schema provider.")
    org: str | None = Field(default=None, description="The pipeline execution org (data vendor), taken from the main input's schema provider. Same stand-in caveat as client.")
    pipeline_config_key: int | None = Field(default=None, description="The key into table pipeline_config for this execution; the jetrules operator uses it to load the rule config.")
    process_name: str | None = Field(default=None, description="The name of the process being executed, from table process_config. Forms the stage-file path together with session_id.")
    read_step_id: str | None = Field(default=None, description="The step id whose stage partitions this step reads as its main input. During sharding it is fixed to 'reducing00', and the code fetches main_input_row_count by that name.")
    session_id: str | None = Field(default=None, description="The unique id of this pipeline execution. Namespaces the stage files (stage_prefix/process_name/session_id/...) and the rows written to the execution-status tables.")
    source_period_key: int | None = Field(default=None, description="The key into table source_period identifying the period the main input file belongs to.")
    sources_config: SourcesConfigSpec | None = Field(default=None, description="Carry-over configuration from table source_config: the main input's columns and domain keys, plus any merged or injected input sources.")
    user_email: str | None = Field(default=None, description="The email of the operator who started the pipeline; recorded in the execution-status tables.")


class ComputePipesConfig(_Base):
    """Complete configuration for a Compute Pipes pipeline"""
    channels: list[ChannelSpec] | None = Field(default=None, description="The channel specs: the row structures shared by the pipes.")
    cluster_config: ClusterSpec | None = Field(default=None, description="Execution sizing of the pipeline: shards, nodes, concurrency.")
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    conditional_pipes_config: list[ConditionalPipeSpec] | None = Field(default=None, description="The authored steps, executed conditionally in order.")
    context: list[ContextSpec] | None = Field(default=None, description="Env vars derived from the file key or from literals.")
    lookup_tables: list[LookupSpec] | None = Field(default=None, description="Lookup tables available to the transformations.")
    metrics_config: MetricsSpec | None = Field(default=None, description="Runtime metrics to emit.")
    output_files: list[OutputFileSpec] | None = Field(default=None, description="Output file locations.")
    output_tables: list[TableSpec] | None = Field(default=None, description="Output tables to load into the JetStore DB.")
    prompt_templates: list[PromptTemplateSpec] | None = Field(default=None, description="Named prompt templates for the ollama operator.")
    reducing_pipes_config: list[list[PipeSpec]] | None = Field(default=None, description="DEPRECATED. The superseded authored form: groups of pipes per reducing step.")
    schema_providers: list[SchemaProviderSpec] | None = Field(default=None, description="Runtime configuration providers for channels and files.")


class ConditionalEnvVariable(_Base):
    """Add env variables using case-else conditions on env variables"""
    case_expr: list[CaseEnvExpression] | None = Field(default=None, description="The when-then legs.")
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    else_expr: list[ExpressionNode] | None = Field(default=None, description="The assignments applied when no leg matches.")


class ConditionalPipeSpec(_Base):
    """Define a stage of a pipeline execution consisting of a compute pipes, executed conditionally."""
    addl_env: list[ConditionalEnvVariable] | None = Field(default=None, description="Env vars added when the step executes.")
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    pipes_config: list[PipeSpec] = Field(description="The pipes executed for this step.")
    step_name: str | None = Field(default=None, description="Name of the step, used in the stage location path.")
    use_ecs_tasks: bool | None = Field(default=None, description="Use_ecs_tasks is true to use ecs fargate task")
    use_ecs_tasks_when: ExpressionNode | None = Field(default=None, description="Use_ecs_tasks_when is an expression as the when property.")
    when: ExpressionNode | None = Field(default=None, description="Condition deciding whether the step executes.")


class ConditionalTransformationSpec(_Base):
    """Configuration to specify a condition to override fields of a transformation operator or replace the transformation altogether."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    then: TransformationSpecOverride | None = Field(default=None, description="The override (type empty) or replacement (type set) applied when the condition holds.")
    when: ExpressionNode = Field(description="When is the condition to evaluate, if true then apply the Then spec.")


class ContextSpec(_Base):
    """ContextSpec: Define env var sourced from the pipeline main file key directory name with pattern expr=value"""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    expr: str | None = Field(default=None, description="The extraction pattern (expr=value) or the literal value.")
    key: str | None = Field(default=None, description="The env var name to define.")
    type: Literal["file_key_component", "partfile_key_component", "value"] | None = Field(default=None, description="The node shape; unary and binary operator nodes carry no type.")


class CsvSourceSpecBase(_Base):
    """CsvSourceSpec: JetStore s3 data source from cpipes stage pipeline"""
    class_name: str | None = Field(default=None, description="Class name used when loading metadata sources for jetrules.")
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    compression: Literal["none", "snappy"] | None = Field(default=None, description="Compression: none, snappy")
    delimiter: int | None = Field(default=None, description="Field delimiter as a code point; default ',' (44). Engine default: 44 (builder).")
    format: Literal["csv", "headerless_csv"] | None = Field(default=None, description="Format: csv, headerless_csv")
    make_empty_source_when_no_files_found: bool | None = Field(default=None, description="MakeEmptyWhenNoFile: Do not make an error when no files are found, make empty source. Default: generate an error when no files are found in s3.")


class CsvSourceSpecCpipes(CsvSourceSpecBase):
    """JetStore s3 data source from cpipes stage pipeline"""
    type: Literal["cpipes"] = Field(description="The source kind. Range: cpipes, csv_file.")
    jets_partition: str | None = Field(default=None, description="Partition label to read (type cpipes).")
    process_name: str | None = Field(default=None, description="Process name of the cpipes pipeline to read from (type cpipes).")
    read_step_id: str | None = Field(default=None, description="Step id in the stage location to read from (type cpipes).")
    session_id: str | None = Field(default=None, description="Session id to read from (type cpipes).")


class CsvSourceSpecCsvFile(CsvSourceSpecBase):
    """JetStore s3 data source from file key"""
    type: Literal["csv_file"] = Field(description="The source kind. Range: cpipes, csv_file.")
    csv_source_file_key: str = Field(description="S3 file key of the csv file (type csv_file).")


class DateFormatLookupSpec(_Base):
    """Configuration for using a lookup table indicating columns containing dates and their formats, typically using the analyze operator output."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    data_classification_column: str | None = Field(default=None, description="Lookup column holding the data classification.")
    date_format_column: str | None = Field(default=None, description="Lookup column holding the column's date format.")
    lookup_key_column: str | None = Field(default=None, description="Lookup column holding the key (column position in the channel).")
    lookup_name: str | None = Field(default=None, description="The lookup table with per-column date formats.")


class DistinctSpec(_Base):
    """Configuration for the distinct transformation operator: specify the composite key."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    distinct_on: list[str] | None = Field(default=None, description="Columns forming the composite key rows are distinct on.")


class DomainKeyInfo(_Base):
    """A composite domain key for one object type: the key_expr column expressions and the object type they hash for."""
    key_expr: list[str] = Field(description="The original function(column) expressions making the domain key.")
    object_type: str | None = Field(default=None, description="Object type associated with the domain key.")


class DomainKeysSpec(_Base):
    """Domain-key configuration: an overriding hashing method applicable to all object types, and per-object-type key info."""
    domain_keys_info: dict[str, DomainKeyInfo] = Field(description="Domain keys keyed by object type.")
    hashing_override: Literal["none", "sha1", "md5"] | None = Field(default=None, description="Overriding hashing method, applicable to all object types.")


class EmbedSpec(_Base):
    """Configuration of the embed transformation operator: model, input template, the column the vector lands in and the request policy."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    connect_timeout_sec: int | None = Field(default=None, description="Connection and tls handshake timeout. Engine default: 10 (builder).")
    error_channel: OutputChannelConfig | None = Field(default=None, description="Channel where row-level errors are reported, on the process_errors channel spec.")
    is_debug: bool | None = Field(default=None, description="Log the prompt and the response of every record.")
    keep_alive: str | None = Field(default=None, description="How long the model stays resident between calls. Engine default: 30m (builder).")
    max_error_count: int | None = Field(default=None, description="Cap on the records reported to the error channel. Engine default: 50 (builder).")
    max_input_count: int | None = Field(default=None, description="Cap on the records sent to the model. A cost guard.")
    max_retry: int | None = Field(default=None, description="Retries on timeout, connection error, 429 and 5xx. Engine default: 2 (builder).")
    model: str = Field(description="The embedding model tag, eg nomic-embed-text. It must be a model whose /api/show capabilities include `embedding`; a generative model refuses the endpoint.")
    on_error: Literal["pass_through", "drop", "fail"] | None = Field(default=None, description="What to do with a record that failed. Engine default: pass_through (builder).")
    options: dict[str, Any] | None = Field(default=None, description="Passed to ollama as options, eg num_ctx.")
    output_mapping: list[InferMappingSpec] | None = Field(default=None, description="Additional mappings, applied on top of the synthesized vector mapping.")
    pool_size: int | None = Field(default=None, description="Concurrent requests to the infer server. Engine default: 1 (validator).")
    prompt_template: str | None = Field(default=None, description="The prompt template, inline.")
    prompt_template_name: str | None = Field(default=None, description="Key of a prompt_templates entry of the document.")
    request_timeout_sec: int | None = Field(default=None, description="Timeout of a single request attempt. Engine default: 120 (builder).")
    retry_wait_sec: int | None = Field(default=None, description="Wait before the first retry, doubled on each attempt. Engine default: 2 (builder).")
    row_key_column: str | None = Field(default=None, description="Column identifying the record in the error reports (row_jets_key).")
    server: OllamaServerSpec | None = Field(default=None, description="How to reach the infer server.")
    truncate: bool | None = Field(default=None, description="Passed to ollama as truncate: truncate an input longer than the model's context.")
    vector_as_rdf_type: str | None = Field(default=None, description="Casts the vector's elements, see CastToRdfType.")
    vector_column: str = Field(description="The column receiving the embedding vector.")


class EntityHint(_Base):
    """Identify the entity (e.g. provider) the column is associated to by using column name fragments, with exclusions."""
    column_name_fragments: list[str] | None = Field(default=None, description="Column-name fragments indicating the entity.")
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    entity: str = Field(description="The entity (eg provider) the column is associated to.")
    exclusion_fragments: list[str] | None = Field(default=None, description="Fragments excluding a column despite a name match.")


class ExpressionNode(_Base):
    """An expression tree node - one class, several structural shapes: a unary operator node (op + arg), a binary operator node (lhs + op + rhs), or a typed leaf (type: select, value, expr_proxy, function, static_list); unary and binary operator nodes carry no type. Self-referential through arg, lhs, rhs, function_arguments, and default. JSON Schema cannot bound the recursion, so the depth budget lives with the caller: the deepest live expression in the corpus is 8 levels, and prompts should bound generation at 12."""
    arg: ExpressionNode | None = Field(default=None, description="The operand of a unary operator node.")
    as_rdf_type: str | None = Field(default=None, description="Casts the leaf value; see CastToRdfType.")
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    default: ExpressionNode | None = Field(default=None, description="Fallback expression used when the evaluation returns an error.")
    expr: str | None = Field(default=None, description="Expr is for leaf nodes, the expression to evaluate: - for Type: select, it is the column name to select or substitute with env var substitution if it contains the char '$'. - for Type: value, it is the value to use or substitute with env var substitution if it contains the char '$'. Required when: absent(expr_pos).")
    expr_env_var_proxy: str | None = Field(default=None, description="Env var whose value is the actual expression, as a json string (type expr_proxy).")
    expr_list: list[str] | None = Field(default=None, description="ExprList is for leaf nodes with multiple values, used for the `in`` operator.")
    expr_pos: int | None = Field(default=None, description="ExprPos is for leaf nodes for Type select, it is the 0-based column position to select, it is an alternative to Expr which is the column name. Required when: absent(expr).")
    function_arguments: list[ExpressionNode] | None = Field(default=None, description="The arguments of a function call (type function).")
    lhs: ExpressionNode | None = Field(default=None, description="The left operand of a binary operator node.")
    max_env_var_substitution: int | None = Field(default=None, description="MaxEnvVarSubstitution indicates how many loop of env substitution to do for Expr containing the char '$', default to 3. For non leaf nodes, Op is the operator: and, or, ==, !=, >, >=, <, <=, etc. Special case for type: expr_proxy, it indicates that the expression is a proxy for another expression, the actual expression is specified by one of: - ExprEnvVarProxy: the expression is specified by an env var, the value of the env var is the actual expression as a json string to evaluate. (more to come) Special case for type: function, it indicates that the expression is a function call, the actual function is specified by Expr, and the arguments are specified by Farg. Default value to use when the evaluation returns error Engine default: 3 (builder).")
    name: str | None = Field(default=None, description="Name is for the special case CaseEnvExpression")
    op: str | None = Field(default=None, description="The operator of a unary or binary node: and, or, ==, !=, >, >=, <, <=, in, etc.")
    rhs: ExpressionNode | None = Field(default=None, description="The right operand of a binary operator node.")
    type: Literal["select", "value", "expr_proxy", "function", "static_list"] | None = Field(default=None, description="The node shape; unary and binary operator nodes carry no type.")


class FieldInfo(_Base):
    """Define the field configuration of a parquet file schema."""
    name: str = Field(description="Field name.")
    nullable: bool | None = Field(default=None, description="Whether nulls are allowed.")
    precision: int | None = Field(default=None, description="Decimal precision.")
    scale: int | None = Field(default=None, description="Decimal scale.")
    type: str | None = Field(default=None, description="The parquet (arrow) type.")


class FilterColumnSpec(_Base):
    """Specify how to filter columns in the input rows before shuffling."""
    column_name: str | None = Field(default=None, description="ColumnName is the name of the column of the lookup table containing the column name to use on the output rows.")
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    lookup_column: str | None = Field(default=None, description="LookupColumn is the name of the column in the lookup table containing column name of the metadata table to filter on.")
    lookup_name: str | None = Field(default=None, description="LookupName is the name of the lookup table containing the column metadata, produced by the analyze operator.")
    retain_on_values: list[str] | None = Field(default=None, description="RetainOnValues is the list of values in the lookup table for LookupColumn to retain, only rows with those values are retained.")


class FilterSpec(_Base):
    """Specify how to filter rows, define a when criteria, max output rows, etc."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    max_output_records: int | None = Field(default=None, description="Cap on the retained rows.")
    row_length_strict: bool | None = Field(default=None, description="RowLengthStrict: when true, will enforce that input row length matches the schema length, otherwise they are filtered.")
    when: ExpressionNode | None = Field(default=None, description="Rows are retained when this evaluates true.")


class FunctionTokenNodeBase(_Base):
    """FunctionTokenNode: Function to identify columns containing dates using a date parser with a collection of date formats."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")


class FunctionTokenNodeParseDate(FunctionTokenNodeBase):
    """Function to identify columns containing dates using a date parser with a collection of date formats."""
    type: Literal["parse_date"] = Field(description="Type: parse_date, parse_double, parse_text MinMaxDateFormat: Date parser, Type: parse_date ParseDateArguments: for Type: parse_date Large_Double: for Type: parse_double")
    parse_date_config: ParseDateSpec = Field(description="Configuration of the parse_date function.")


class FunctionTokenNodeParseDouble(FunctionTokenNodeBase):
    """Function to identify columns containing numeric values."""
    type: Literal["parse_double"] = Field(description="Type: parse_date, parse_double, parse_text MinMaxDateFormat: Date parser, Type: parse_date ParseDateArguments: for Type: parse_date Large_Double: for Type: parse_double")
    large_double: float | None = Field(default=None, description="Threshold flagging large numeric values (parse_double).")


class FunctionTokenNodeParseText(FunctionTokenNodeBase):
    """Function to identify columns containing textual data, fallback to parse_date and parse_double functions."""
    type: Literal["parse_text"] = Field(description="Type: parse_date, parse_double, parse_text MinMaxDateFormat: Date parser, Type: parse_date ParseDateArguments: for Type: parse_date Large_Double: for Type: parse_double")


class GroupBySpec(_Base):
    """Specify how rows are grouped, one of: domain_key, group_by_name, group_by_pos or group_by_count."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    domain_key: str | None = Field(default=None, description="Compute the composite key from the domain key of this object type.")
    group_by_count: int | None = Field(default=None, description="Group every n records.")
    group_by_name: list[str] | None = Field(default=None, description="Columns forming the composite key, by name.")
    group_by_pos: list[int] | None = Field(default=None, description="Columns forming the composite key, by position (0-based).")
    is_debug: bool | None = Field(default=None, description="Log trace info.")


class HashExpression(_Base):
    """Configuration for hashing values from columns."""
    alternate_composite_expr: list[str] | None = Field(default=None, description="AlternateCompositeExpr is used when Expr or CompositeExpr returns nil or empty. MultiStepShardingMode values: 'limited_range', 'full_range' or empty. NoPartitions indicated not to assign the hash to a partition (no modulo operation). NbrJetsPartitions is the number of partitions to use for the hash operator when NoPartitions is false. MaxNbrJetsPartitions use the minimum between the cluster nbr of partitions and this setting provided the NoPartitions is false. NbrJetsPartitions takes precedence over MaxNbrJetsPartitions when both are provided. ComputeDomainKey flag indicate to compute the domain key rather than a simple hash. This consider the hashing algo used and delimiter between the key components.")
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    composite_expr: list[str] | None = Field(default=None, description="The columns of the composite hash.")
    compute_domain_key: bool | None = Field(default=None, description="Compute the domain key rather than a simple hash.")
    domain_key: str | None = Field(default=None, description="DomainKey is specified as an object_type. DomainKeysJson provides the mapping between domain keys and columns.")
    expr: str | None = Field(default=None, description="The single column to hash.")
    max_nbr_jets_partitions: int | str | None = Field(default=None, description="Cap: the minimum of the cluster's partitions and this value.")
    multi_step_sharding_mode: Literal["limited_range", "full_range"] | None = Field(default=None, description="Sharding mode: limited_range or full_range.")
    nbr_jets_partitions: int | str | None = Field(default=None, description="Nbr of partitions for the hash, when no_partitions is false.")
    no_partitions: bool | None = Field(default=None, description="Do not map the hash to a partition (no modulo).")


class HighFreqSpec(_Base):
    """Configuration for the high_freq transformation operator: top percentile and rank to keep, optional regex to apply on input data."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    key_re: str | None = Field(default=None, description="Optional regex applied to the input values.")
    name: str = Field(description="The column classification this spec applies to.")
    top_pct: int | None = Field(default=None, description="Retain the distinct values accounting for the top n percent of the total count.")
    top_rank: int | None = Field(default=None, description="Retain the top n percent of distinct values, by descending frequency.")


class InferMappingSpec(_Base):
    """Maps one element of the model response to a column of the output record."""
    as_rdf_type: str | None = Field(default=None, description="Casts the value; see CastToRdfType.")
    column: str = Field(description="The column to set. Must be a column of the shared channel spec.")
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    default: str | None = Field(default=None, description="Value to use when the path is absent or null.")
    path: str | None = Field(default=None, description="Dot path into the parsed json, eg codes.0.icd10. Empty takes the whole value.")
    required: bool | None = Field(default=None, description="An absent or null value is a row-level error.")
    source: Literal["response", "raw_response", "envelope", "thinking", "model_name"] | None = Field(default=None, description="What the mapping reads from. Engine default: response (builder).")


class InputChannelConfigBase(_Base):
    """InputChannelConfig: In memory channel chaining two compute pipes."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    has_grouped_rows: bool | None = Field(default=None, description="HasGroupedRows indicates that the channel contains grouped rows, most likely from the group_by or merge operator.")
    name: str = Field(description="The channel name; must be unique across the pipes of a step.")


class InputChannelConfigMemory(InputChannelConfigBase):
    """In memory channel chaining two compute pipes."""
    type: Literal["memory"] = Field(default="memory", description="The channel type. Range: memory (default), input, stage, generator. Engine default: memory (validator).")


class InputChannelConfigInput(InputChannelConfigBase):
    """Input channel (source) of the first compute pipe in a chain."""
    type: Literal["input"] = Field(description="The channel type. Range: memory (default), input, stage, generator.")
    bad_rows_config: BadRowsSpec | None = Field(default=None, description="BadRowsConfig: Specify how to handle bad rows.")
    blank_field_markers: BlankFieldMarkersSpec | None = Field(default=None, description="Marker texts treated as blank (null) values.")
    bucket: str | None = Field(default=None, description="S3 bucket holding the file.")
    compression: str | None = Field(default=None, description="Compression: none, snappy (parquet: always snappy)")
    delimiter: int | None = Field(default=None, description="Field delimiter, as a code point (a json number). Engine default: 44 (builder).")
    detect_cr_as_eol: bool | None = Field(default=None, description="DetectCrAsEol: Detect if \\r is used as eol (format: csv,headerless_csv)")
    detect_encoding: bool | None = Field(default=None, description="DetectEncoding: Detect file encoding (limited) for text file format")
    discard_file_headers: bool | None = Field(default=None, description="DiscardFileHeaders: when true, discard the headers from the input file (typically for csv format).")
    domain_class: str | None = Field(default=None, description="Domain class of the records.")
    domain_keys: dict[str, str | list[str]] | None = Field(default=None, description="Domain key configuration for the records.")
    drop_excedent_headers: bool | None = Field(default=None, description="Drop headers beyond the configured columns.")
    encoding: str | None = Field(default=None, description="Character encoding of the file.")
    enforce_row_max_length: bool | None = Field(default=None, description="Fail rows with extra characters past the last field (text formats).")
    enforce_row_min_length: bool | None = Field(default=None, description="Require all columns in the input record; otherwise missing columns are null (text formats).")
    eol_byte: int | None = Field(default=None, description="EolByte: Byte to use as eol (format: csv,headerless_csv)")
    fail_on_empty_column_name: bool | None = Field(default=None, description="Fail when a column name is empty (csv), preventing a data row from standing as headers.")
    fixed_width_columns_csv: str | None = Field(default=None, description="Column names and offsets for the fixed_width format, as csv text.")
    format: str | None = Field(default=None, description="Format: csv, headerless_csv, etc.")
    input_format_data_json: str | None = Field(default=None, description="Format-specific reader config as json, eg the sheet name for xlsx.")
    is_part_files: bool | None = Field(default=None, description="The file key is a directory of part files rather than a single file.")
    lookback_periods: str | None = Field(default=None, description="Reads earlier source periods when the input is selected by explicit file_key from the stage area: the key is expanded once per period, substituting ${PERIOD_ID}, and the file listings are concatenated. Format: 'N' reads the current period plus N periods back; 'offset:last' reads current-offset back through current-last ('1:1' is the previous period only). Values may reference env vars; ${PERIOD_ID_TYPE} must name the current period id in the env.")
    multi_columns_input: bool | None = Field(default=None, description="MultiColumnsInput: Indicate that input file must have multiple columns, this is used to detect if the wrong delimiter is used (csv,headerless_csv)")
    no_quotes: bool | None = Field(default=None, description="Never quote records on the csv writer, even when a record contains a quote.")
    parquet_schema: ParquetSchemaInfo | None = Field(default=None, description="Schema of the parquet file; typically populated by JetStore from the input file.")
    read_batch_size: int | None = Field(default=None, description="ReadBatchSize: nbr of rows to read per record (format: parquet)")
    read_date_layout: str | None = Field(default=None, description="Date layout used to parse dates on read.")
    reorder_columns_on_read: list[int] | None = Field(default=None, description="Column positions used to reorder the input columns on read.")
    sampling_max_count: int | None = Field(default=None, description="Cap on the sampled rows.")
    sampling_rate: int | None = Field(default=None, description="Read one row in n.")
    trim_columns: bool | None = Field(default=None, description="Trim whitespace around column values.")
    use_lazy_quotes: bool | None = Field(default=None, description="Tolerant csv quote handling; see csv.NewReader.")
    use_lazy_quotes_special: bool | None = Field(default=None, description="Variant of use_lazy_quotes; see csv.NewReader.")
    variable_fields_per_record: bool | None = Field(default=None, description="Allow a variable number of fields per record; see csv.NewReader.")


class InputChannelConfigStage(InputChannelConfigBase):
    """Reads the partition files written to the stage location by an earlier step."""
    type: Literal["stage"] = Field(description="The channel type. Range: memory (default), input, stage, generator.")
    bad_rows_config: BadRowsSpec | None = Field(default=None, description="BadRowsConfig: Specify how to handle bad rows.")
    blank_field_markers: BlankFieldMarkersSpec | None = Field(default=None, description="Marker texts treated as blank (null) values.")
    bucket: str | None = Field(default=None, description="Bucket holding the staged files. Engine default: jetstore_bucket (validator).")
    compression: str | None = Field(default=None, description="Compression of the staged files. Engine default: snappy (validator).")
    delimiter: int | None = Field(default=None, description="Field delimiter of the staged files. Engine default: 44 (validator).")
    detect_cr_as_eol: bool | None = Field(default=None, description="DetectCrAsEol: Detect if \\r is used as eol (format: csv,headerless_csv)")
    detect_encoding: bool | None = Field(default=None, description="DetectEncoding: Detect file encoding (limited) for text file format")
    discard_file_headers: bool | None = Field(default=None, description="DiscardFileHeaders: when true, discard the headers from the input file (typically for csv format).")
    domain_class: str | None = Field(default=None, description="Domain class of the records.")
    domain_keys: dict[str, str | list[str]] | None = Field(default=None, description="Domain key configuration for the records.")
    drop_excedent_headers: bool | None = Field(default=None, description="Drop headers beyond the configured columns.")
    encoding: str | None = Field(default=None, description="Character encoding of the file.")
    enforce_row_max_length: bool | None = Field(default=None, description="Fail rows with extra characters past the last field (text formats).")
    enforce_row_min_length: bool | None = Field(default=None, description="Require all columns in the input record; otherwise missing columns are null (text formats).")
    eol_byte: int | None = Field(default=None, description="EolByte: Byte to use as eol (format: csv,headerless_csv)")
    fail_on_empty_column_name: bool | None = Field(default=None, description="Fail when a column name is empty (csv), preventing a data row from standing as headers.")
    file_key: str | None = Field(default=None, description="S3 object key of the file, or its directory when is_part_files; on output configs this is the output location.")
    fixed_width_columns_csv: str | None = Field(default=None, description="Column names and offsets for the fixed_width format, as csv text.")
    format: str | None = Field(default=None, description="File format of the staged files. Engine default: headerless_csv (validator).")
    input_format_data_json: str | None = Field(default=None, description="Format-specific reader config as json, eg the sheet name for xlsx.")
    is_part_files: bool | None = Field(default=None, description="The file key is a directory of part files rather than a single file.")
    lookback_periods: str | None = Field(default=None, description="Reads earlier source periods when the input is selected by explicit file_key from the stage area: the key is expanded once per period, substituting ${PERIOD_ID}, and the file listings are concatenated. Format: 'N' reads the current period plus N periods back; 'offset:last' reads current-offset back through current-last ('1:1' is the previous period only). Values may reference env vars; ${PERIOD_ID_TYPE} must name the current period id in the env.")
    merge_channels: list[InputChannelConfig] | None = Field(default=None, description="Input channels merged into this one.")
    multi_columns_input: bool | None = Field(default=None, description="MultiColumnsInput: Indicate that input file must have multiple columns, this is used to detect if the wrong delimiter is used (csv,headerless_csv)")
    no_quotes: bool | None = Field(default=None, description="Never quote records on the csv writer, even when a record contains a quote.")
    parquet_schema: ParquetSchemaInfo | None = Field(default=None, description="Schema of the parquet file; typically populated by JetStore from the input file.")
    read_batch_size: int | None = Field(default=None, description="ReadBatchSize: nbr of rows to read per record (format: parquet)")
    read_date_layout: str | None = Field(default=None, description="Date layout used to parse dates on read.")
    read_partition_id: str | None = Field(default=None, description="Partition to read, when reading a single partition.")
    read_session_id: str | None = Field(default=None, description="Session id to read from, when reading another session's stage data.")
    read_step_id: str | None = Field(default=None, description="Step id under the stage location to read from.")
    reorder_columns_on_read: list[int] | None = Field(default=None, description="Column positions used to reorder the input columns on read.")
    sampling_max_count: int | None = Field(default=None, description="Cap on the sampled rows.")
    sampling_rate: int | None = Field(default=None, description="Read one row in n.")
    schema_provider: str | None = Field(default=None, description="Key of a schema_providers entry; its settings are synced onto this channel.")
    trim_columns: bool | None = Field(default=None, description="Trim whitespace around column values.")
    use_lazy_quotes: bool | None = Field(default=None, description="Tolerant csv quote handling; see csv.NewReader.")
    use_lazy_quotes_special: bool | None = Field(default=None, description="Variant of use_lazy_quotes; see csv.NewReader.")
    variable_fields_per_record: bool | None = Field(default=None, description="Allow a variable number of fields per record; see csv.NewReader.")


class InputChannelConfigGenerator(InputChannelConfigBase):
    """Input channel (source) that generate rows based on count."""
    type: Literal["generator"] = Field(description="The channel type. Range: memory (default), input, stage, generator.")
    nbr_nodes: int | str | None = Field(default=None, description="Nbr of nodes for the generator; int or string with env var substitution.")
    nbr_rows: int | str | None = Field(default=None, description="Nbr of rows to generate; int or string with env var substitution.")


class InputSourceSpec(_Base):
    """One input source: its original and current column names, domain class and domain keys."""
    domain_class: str | None = Field(default=None, description="The domain class of the input when source_type is domain_table, from domain_keys_registry or the schema provider / input_source_spec. Does not apply to source_type file - the file needs to be mapped first.")
    domain_keys_spec: DomainKeysSpec | None = Field(default=None, description="The domain-keys spec of the input: from source_config or the main schema provider for source_type file; from domain_keys_registry or the schema provider / input_source_spec for source_type domain_table.")
    input_columns: list[str] | None = Field(default=None, description="The uniquefied version of original_input_columns (duplicate names made unique); for a sharding step it also includes the part-file key columns.")
    original_input_columns: list[str] | None = Field(default=None, description="The original column names of the input file, before duplicate names are uniquefied.")


class JetrulesSpec(_Base):
    """Configuration for the JetRules transformation operator: rule process name, input rdf type, worker pool size, etc."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    current_source_period: int | None = Field(default=None, description="CurrentSourcePeriod is the source period key to use for this process.")
    current_source_period_date: str | None = Field(default=None, description="CurrentSourcePeriodDate is the source period date (aka file period date) to use this process.")
    current_source_period_type: str | None = Field(default=None, description="CurrentSourcePeriodType is the source period type (day, week or, month) to use for this process.")
    error_channel: OutputChannelConfig | None = Field(default=None, description="ErrorChannel specify the channel to write the errors and exported triples from JetRules processing.")
    input_rdf_type: str | None = Field(default=None, description="InputRdfType is the rdf type (class name) of the input records.")
    is_debug: bool | None = Field(default=None, description="IsDebug when true enable debug mode for jetrules processing. MaxLooping overrides the value in the jetrules metastore.")
    max_error_count: int | None = Field(default=None, description="Cap on the errors reported to the log and the error channel. Errors are logged even when no error_channel is configured. Engine default: 20 (builder).")
    max_input_count: int | None = Field(default=None, description="MaxInputCount is the max nbr of input records to process.")
    max_looping: int | None = Field(default=None, description="Overrides the max looping value from the jetrules metastore.")
    max_rete_sessions_saved: int | None = Field(default=None, description="MaxReteSessionsSaved is the max nbr of rete sessions to save in err table.")
    metadata_input_sources: list[CsvSourceSpec] | None = Field(default=None, description="MetadataInputSources provide the list of csv sources to load as metadata input sources for jetrules processing.")
    on_error: Literal["pass_through", "drop", "fail"] | None = Field(default=None, description="What to do when a record bundle fails rule execution (ExecuteRules error, max-loop reached, or jets:exception). Range: pass_through (default, the session data is still extracted), drop (the bundle's output is discarded), fail (the pipeline is aborted). Engine default: pass_through (builder).")
    output_channels: list[OutputChannelConfig] = Field(description="OutputChannels specify the output channels to write the extracted entities from JetRules")
    pool_size: int | None = Field(default=None, description="PoolSize is the nbr of worker pool size. Engine default: 1 (validator).")
    process_name: str | None = Field(default=None, description="ProcessName is the jetrules process name to use.")
    rule_config: list[dict[str, Any]] | None = Field(default=None, description="RuleConfig provide additional configuration for jetrules processing.")
    use_jet_rules_go: bool | None = Field(default=None, description="UseJetRulesGo when true use the jetrules go engine.")
    use_jet_rules_native: bool | None = Field(default=None, description="UseJetRulesNative when true use the jetrules native engine.")


class KeywordTokenNode(_Base):
    """Ratio of a column's data matching a set of keywords."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    keywords: list[str] | None = Field(default=None, description="Keywords identifying it.")
    name: str = Field(description="The classification token.")


class LookupColumnSpecBase(_Base):
    """LookupColumnSpec: Select returns the value of input column.Select the value from an input column (lookup key) or lookup table row (lookup values)"""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    expr: str = Field(description="The input column (select) or the literal value (value).")
    name: str | None = Field(default=None, description="Column name (lookup key or lookup value).")


class LookupColumnSpecSelect(LookupColumnSpecBase):
    """Select returns the value of input column.Select the value from an input column (lookup key) or lookup table row (lookup values)"""
    type: Literal["select"] = Field(description="How the value is obtained. Range: select, value.")


class LookupColumnSpecValue(LookupColumnSpecBase):
    """Set to a constant value (lookup values)"""
    type: Literal["value"] = Field(description="How the value is obtained. Range: select, value.")
    max_env_var_substitution: int | None = Field(default=None, description="Rounds of env var substitution for expr (type value).")


class LookupSpecBase(_Base):
    """LookupSpec: Lookup table configuration with data sourced from JetStore DB (postgres)"""
    columns: list[TableColumnSpec] | None = Field(default=None, description="Column metadata of the lookup table.")
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    key: str = Field(description="The lookup table key referenced by the transformations.")
    lookup_key: list[str] | None = Field(default=None, description="Columns forming the lookup key.")
    lookup_values: list[str] | None = Field(default=None, description="Columns returned as the lookup values.")


class LookupSpecSqlLookup(LookupSpecBase):
    """Lookup table configuration with data sourced from JetStore DB (postgres)"""
    type: Literal["sql_lookup"] = Field(description="The source kind. Range: sql_lookup, s3_csv_lookup.")
    query: str = Field(description="The sql query sourcing the table (sql_lookup).")


class LookupSpecS3CsvLookup(LookupSpecBase):
    """Lookup table configuration with data sourced from JetStore s3 data source"""
    type: Literal["s3_csv_lookup"] = Field(description="The source kind. Range: sql_lookup, s3_csv_lookup.")
    csv_source: CsvSourceSpec = Field(description="The s3 csv source of the table (s3_csv_lookup).")


class LookupTokenNode(_Base):
    """Ratio of column's data matching reference in lookup tables"""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    key_re: str | None = Field(default=None, description="Regex extracting the key from the value before lookup.")
    lookup_name: str = Field(description="The lookup table used for matching.")
    multi_tokens_match: list[MultiTokensNode] | None = Field(default=None, description="MultiTokensMatch: Matching composite values, separated by space(s)")
    tokens: list[str] | None = Field(default=None, description="The tokens whose occurrences are counted: each input value is looked up in the lookup table (the whole value, or the key_re capture group) and every returned token in this list has its count incremented.")


class MapExpression(_Base):
    """Configuration details for mapping: cleansing functions, type casting, default value"""
    argument: str | None = Field(default=None, description="Argument of the cleansing function.")
    cleansing_function: str | None = Field(default=None, description="The cleansing function applied to the input value.")
    code_value_mapping: dict[str, str] | None = Field(default=None, description="Map of input code to output value.")
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    default: str | None = Field(default=None, description="Value to use when the input is empty or cleansing fails.")
    err_msg: str | None = Field(default=None, description="Error message reported when the mapping fails.")
    rdf_type: str | None = Field(default=None, description="Type conversion applied to the output value.")


class MapRecordSpec(_Base):
    """Optional configuration for map_record operator: log trace info, read column mapping from db table."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    error_channel: OutputChannelConfig | None = Field(default=None, description="Channel where mapping errors are reported.")
    fail_on_error: bool | None = Field(default=None, description="Interrupt the pipeline on a mapping error.")
    file_mapping_table_name: str | None = Field(default=None, description="FileMappingTableName is the name of the file_mapping table to use for the mapping. The file_mapping table specification is optional. When present, columns transformations are created from the file_mapping table entries.")
    is_debug: bool | None = Field(default=None, description="Log trace info.")
    max_error_count: int | None = Field(default=None, description="Cap on the errors reported to the log and the error channel. Errors are logged even when no error_channel is configured. Engine default: 20 (builder).")
    on_error: Literal["pass_through", "drop", "fail"] | None = Field(default=None, description="What to do with a record whose column transformation failed. Range: pass_through (default, the record is sent to the output), drop, fail. fail_on_error is the legacy spelling of on_error: fail, honoured when on_error is not set. Engine default: pass_through (builder).")


class MergeFileSpec(_Base):
    """Configuration for merging multiple part files into a single output."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    first_partition_has_headers: bool | None = Field(default=None, description="FirstPartitionHasHeaders: when true, the first partitions has the headers (considered for csv to determine if use s3 multipart copy).")


class MergeSpec(_Base):
    """Configuration for the merge transformation operator: specify the channels to merge and how to group them."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    is_debug: bool | None = Field(default=None, description="Log trace info.")
    main_group_by: GroupBySpec = Field(description="Grouping of the main input channel.")
    merge_group_by: list[GroupBySpec] | None = Field(default=None, description="Grouping of each merged channel.")


class Metric(_Base):
    """Specific metric configuration to emit during pipeline execution"""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    name: Literal["alloc_mb", "total_alloc_mb", "sys_mb", "nbr_gc"] | None = Field(default=None, description="The runtime metric: alloc_mb, total_alloc_mb, sys_mb or nbr_gc.")
    type: Literal["runtime"] | None = Field(default=None, description="The metric kind; runtime is the only value.")


class MetricsSpec(_Base):
    """Configure metrics to emit during pipeline execution"""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    report_interval_sec: int | None = Field(default=None, description="Seconds between metric reports.")
    runtime_metrics: list[Metric] | None = Field(default=None, description="The metrics to emit.")


class MultiTokensNode(_Base):
    """Identifies a value made of multiple tokens (e.g. full name): the value is split on spaces and punctuation and each split value must match at least one token."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    name: str = Field(description="The name of the feature, ie the output column name.")
    nbr_tokens: int = Field(description="The number of words a composite value must split into for this node to be considered: the value is split on spaces (single-letter words and trailing commas dropped) and matches when every word maps, via the lookup table, to one of the node's tokens.")
    tokens: list[str] = Field(description="Each split value must match at least one token.")


class OllamaServerSpec(_Base):
    """Specify how to reach the infer server: url, headers"""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    headers: dict[str, str] | None = Field(default=None, description="Headers are additional request headers, optional.")
    url: str | None = Field(default=None, description="Url is the base url of the infer server, eg http://my-elb:11434. It is resolved in this order: this property (with cpipes env var substitution), then $JETS_INFER_URL from the cpipes env, then the JETS_INFER_URL environment variable - which is set on the deployed containers when the stack is built with BUILD_INFER_SERVICE.")


class OllamaSpec(_Base):
    """Configuration of the ollama transformation operator: model, prompt, response mapping and the request policy."""
    api: Literal["generate", "chat"] | None = Field(default=None, description="The ollama api to call. Engine default: generate (builder).")
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    connect_timeout_sec: int | None = Field(default=None, description="Connection and tls handshake timeout. Engine default: 10 (builder).")
    disable_strip_code_fences: bool | None = Field(default=None, description="Turn off the removal of markdown code fences around the response.")
    error_channel: OutputChannelConfig | None = Field(default=None, description="Channel where row-level errors are reported, on the process_errors channel spec.")
    is_debug: bool | None = Field(default=None, description="Log the prompt and the response of every record.")
    keep_alive: str | None = Field(default=None, description="How long the model stays resident between calls. Engine default: 30m (builder).")
    max_error_count: int | None = Field(default=None, description="Cap on the records reported to the error channel. Engine default: 50 (builder).")
    max_input_count: int | None = Field(default=None, description="Cap on the records sent to the model. A cost guard.")
    max_retry: int | None = Field(default=None, description="Retries on timeout, connection error, 429 and 5xx. Engine default: 2 (builder).")
    model: str = Field(description="The model tag to use, eg llama3.1:8b.")
    on_error: Literal["pass_through", "drop", "fail"] | None = Field(default=None, description="What to do with a record that failed. Engine default: pass_through (builder).")
    options: dict[str, Any] | None = Field(default=None, description="Passed to ollama as options, eg temperature, num_ctx, seed, num_predict.")
    output_mapping: list[InferMappingSpec] = Field(description="How the response maps onto the record's columns.")
    pool_size: int | None = Field(default=None, description="Concurrent requests to the infer server. Engine default: 1 (validator).")
    prompt_template: str | None = Field(default=None, description="The prompt template, inline.")
    prompt_template_name: str | None = Field(default=None, description="Key of a prompt_templates entry of the document.")
    provenance_schema_name: str | None = Field(default=None, description="Names a provenance schema of the workspace, provenance/<name>.pv.json, turning on the per-field provenance check of jets/agentic/briefing.")
    request_timeout_sec: int | None = Field(default=None, description="Timeout of a single request attempt. Engine default: 120 (builder).")
    response_format: str | dict[str, Any] | None = Field(default=None, description="Passed to ollama as format: the string \"json\" or a json schema.")
    retry_wait_sec: int | None = Field(default=None, description="Wait before the first retry, doubled on each attempt. Engine default: 2 (builder).")
    row_key_column: str | None = Field(default=None, description="Column identifying the record in the error reports (row_jets_key).")
    server: OllamaServerSpec | None = Field(default=None, description="How to reach the infer server.")
    system_prompt: str | None = Field(default=None, description="The system message.")
    think: bool | None = Field(default=None, description="Passed to ollama as think, for reasoning models.")


class OutputChannelConfigBase(_Base):
    """OutputChannelConfig: Configuration for in-memory channel to pass records between transformation operators."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")


class OutputChannelConfigMemory(OutputChannelConfigBase):
    """Configuration for in-memory channel to pass records between transformation operators."""
    type: Literal["memory"] = Field(default="memory", description="The channel type. Range: memory (default), stage, output, sql. Engine default: memory (validator).")
    channel_spec_name: str = Field(description="Key of the channels entry providing this channel's spec, when it differs from name.")
    name: str = Field(description="Name: output channel name, required (must exist in the channels section of the config document) Format: file format, range values: csv, headerless_csv, fixed_width. NbrRowsInRecord: nbr of rows in record (applicable to format: parquet) Compression: none, snappy (default). Does not apply to parquet format (always snappy). UseInputParquetSchema to use the same schema as the input file. UseOriginalHeaders to use the headers from the input file (csv only). Must have save_parquet_schema = true in the cpipes first input_channel. OutputLocation: jetstore_s3_schema_events, jetstore_s3_input, jetstore_s3_output (default), or custom location. When OutputLocation is jetstore_s3_input it will also write to the input bucket. When using jetstore_s3_input and jetstore_s3_schema_events you must specify WriteStepId to specify the step id in stage location to output the file. When OutputLocation uses a custom location, it replaces KeyPrefix and FileName. OutputLocation must ends with \"/\" if we want to use default file name (i.e. OutputLocation does not include the file name).")


class OutputChannelConfigStage(OutputChannelConfigBase):
    """Configuration for a channel writing to JetStore stage s3 location."""
    type: Literal["stage"] = Field(description="The channel type. Range: memory (default), stage, output, sql.")
    bucket: str | None = Field(default=None, description="S3 bucket holding the file.")
    channel_spec_name: str = Field(description="Key of the channels entry providing this channel's spec, when it differs from name.")
    compression: Literal["none", "snappy"] | None = Field(default=None, description="Compression: none, snappy (default). Does not apply to parquet format (always snappy). UseInputParquetSchema to use the same schema as the input file. UseOriginalHeaders to use the headers from the input file (csv only). Must have save_parquet_schema = true in the cpipes first input_channel. OutputLocation: jetstore_s3_schema_events, jetstore_s3_input, jetstore_s3_output (default), or custom location. When OutputLocation is jetstore_s3_input it will also write to the input bucket. When using jetstore_s3_input and jetstore_s3_schema_events you must specify WriteStepId to specify the step id in stage location to output the file. When OutputLocation uses a custom location, it replaces KeyPrefix and FileName. OutputLocation must ends with \"/\" if we want to use default file name (i.e. OutputLocation does not include the file name).")
    delimiter: int | None = Field(default=None, description="Field delimiter, as a code point (a json number). Engine default: 44 (validator).")
    domain_class: str | None = Field(default=None, description="Domain class of the records.")
    domain_keys: dict[str, str | list[str]] | None = Field(default=None, description="Domain key configuration for the records.")
    encoding: str | None = Field(default=None, description="Character encoding of the file.")
    file_key: str | None = Field(default=None, description="S3 object key of the file, or its directory when is_part_files; on output configs this is the output location. Required when: absent(write_step_id).")
    file_name: str | None = Field(default=None, description="File name of the output file (type output).")
    fixed_width_columns_csv: str | None = Field(default=None, description="Column names and offsets for the fixed_width format, as csv text.")
    format: Literal["csv", "headerless_csv", "fixed_width", "parquet"] | None = Field(default=None, description="Format: file format, range values: csv, headerless_csv, fixed_width.")
    key_prefix: str | None = Field(default=None, description="KeyPrefix is optional, default to $PATH_FILE_KEY. Use $CURRENT_PARTITION_LABEL in KeyPrefix and FileName to substitute with current partition label. Other available env substitution (this is not comprehensive list, any defined env var can be used): $FILE_KEY main input file key. $SESSIONID current session id. ${REQUEST_ID} current request id. $PROCESS_NAME current process name. $PATH_FILE_KEY file key path portion. $NAME_FILE_KEY file key file name portion (empty when in part files mode). $SHARD_ID current node id. $JETS_PARTITION_LABEL current node partition label.")
    name: str = Field(description="Name: output channel name, required (must exist in the channels section of the config document) Format: file format, range values: csv, headerless_csv, fixed_width. NbrRowsInRecord: nbr of rows in record (applicable to format: parquet) Compression: none, snappy (default). Does not apply to parquet format (always snappy). UseInputParquetSchema to use the same schema as the input file. UseOriginalHeaders to use the headers from the input file (csv only). Must have save_parquet_schema = true in the cpipes first input_channel. OutputLocation: jetstore_s3_schema_events, jetstore_s3_input, jetstore_s3_output (default), or custom location. When OutputLocation is jetstore_s3_input it will also write to the input bucket. When using jetstore_s3_input and jetstore_s3_schema_events you must specify WriteStepId to specify the step id in stage location to output the file. When OutputLocation uses a custom location, it replaces KeyPrefix and FileName. OutputLocation must ends with \"/\" if we want to use default file name (i.e. OutputLocation does not include the file name).")
    nbr_rows_in_record: int | None = Field(default=None, description="NbrRowsInRecord: nbr of rows in record (applicable to format: parquet)")
    no_quotes: bool | None = Field(default=None, description="Never quote records on the csv writer, even when a record contains a quote.")
    output_encoding: str | None = Field(default=None, description="Character encoding of the output file.")
    output_encoding_same_as_input: bool | None = Field(default=None, description="Use the input file's encoding for the output.")
    parquet_schema: ParquetSchemaInfo | None = Field(default=None, description="Schema of the parquet file; typically populated by JetStore from the input file.")
    put_headers_on_first_partition: bool | None = Field(default=None, description="Write the header line on the first partition file.")
    quote_all_records: bool | None = Field(default=None, description="Quote every record on the csv writer.")
    schema_provider: str | None = Field(default=None, description="Key of a schema_providers entry; alternative to format (types stage, output).")
    use_input_parquet_schema: bool | None = Field(default=None, description="Use the same parquet schema as the input file (types stage, output).")
    use_original_headers: bool | None = Field(default=None, description="Use the original headers that came with the input file (csv only) rather than the uniquefied headers JetStore processes with; applies to type output and to the stage channels leading to an output channel.")
    write_date_layout: str | None = Field(default=None, description="Date layout used to format dates on write.")
    write_step_id: str | None = Field(default=None, description="Step id in the stage location to write to (type stage). Required when: absent(file_key).")


class OutputChannelConfigOutput(OutputChannelConfigBase):
    """Configuration for a channel writing to a specified s3 location."""
    type: Literal["output"] = Field(description="The channel type. Range: memory (default), stage, output, sql.")
    bucket: str | None = Field(default=None, description="S3 bucket holding the file.")
    channel_spec_name: str = Field(description="Key of the channels entry providing this channel's spec, when it differs from name.")
    compression: Literal["none", "snappy"] | None = Field(default=None, description="Compression: none, snappy (default). Does not apply to parquet format (always snappy). UseInputParquetSchema to use the same schema as the input file. UseOriginalHeaders to use the headers from the input file (csv only). Must have save_parquet_schema = true in the cpipes first input_channel. OutputLocation: jetstore_s3_schema_events, jetstore_s3_input, jetstore_s3_output (default), or custom location. When OutputLocation is jetstore_s3_input it will also write to the input bucket. When using jetstore_s3_input and jetstore_s3_schema_events you must specify WriteStepId to specify the step id in stage location to output the file. When OutputLocation uses a custom location, it replaces KeyPrefix and FileName. OutputLocation must ends with \"/\" if we want to use default file name (i.e. OutputLocation does not include the file name).")
    delimiter: int | None = Field(default=None, description="Field delimiter, as a code point (a json number). Engine default: 44 (validator).")
    domain_class: str | None = Field(default=None, description="Domain class of the records.")
    domain_keys: dict[str, str | list[str]] | None = Field(default=None, description="Domain key configuration for the records.")
    encoding: str | None = Field(default=None, description="Character encoding of the file.")
    file_name: str | None = Field(default=None, description="File name of the output file (type output).")
    fixed_width_columns_csv: str | None = Field(default=None, description="Column names and offsets for the fixed_width format, as csv text.")
    format: Literal["csv", "headerless_csv", "fixed_width", "parquet"] | None = Field(default=None, description="Format: file format, range values: csv, headerless_csv, fixed_width. Required when: absent(schema_provider).")
    key_prefix: str | None = Field(default=None, description="KeyPrefix is optional, default to $PATH_FILE_KEY. Use $CURRENT_PARTITION_LABEL in KeyPrefix and FileName to substitute with current partition label. Other available env substitution (this is not comprehensive list, any defined env var can be used): $FILE_KEY main input file key. $SESSIONID current session id. ${REQUEST_ID} current request id. $PROCESS_NAME current process name. $PATH_FILE_KEY file key path portion. $NAME_FILE_KEY file key file name portion (empty when in part files mode). $SHARD_ID current node id. $JETS_PARTITION_LABEL current node partition label.")
    name: str = Field(description="Name: output channel name, required (must exist in the channels section of the config document) Format: file format, range values: csv, headerless_csv, fixed_width. NbrRowsInRecord: nbr of rows in record (applicable to format: parquet) Compression: none, snappy (default). Does not apply to parquet format (always snappy). UseInputParquetSchema to use the same schema as the input file. UseOriginalHeaders to use the headers from the input file (csv only). Must have save_parquet_schema = true in the cpipes first input_channel. OutputLocation: jetstore_s3_schema_events, jetstore_s3_input, jetstore_s3_output (default), or custom location. When OutputLocation is jetstore_s3_input it will also write to the input bucket. When using jetstore_s3_input and jetstore_s3_schema_events you must specify WriteStepId to specify the step id in stage location to output the file. When OutputLocation uses a custom location, it replaces KeyPrefix and FileName. OutputLocation must ends with \"/\" if we want to use default file name (i.e. OutputLocation does not include the file name).")
    nbr_rows_in_record: int | None = Field(default=None, description="NbrRowsInRecord: nbr of rows in record (applicable to format: parquet)")
    no_quotes: bool | None = Field(default=None, description="Never quote records on the csv writer, even when a record contains a quote.")
    output_encoding: str | None = Field(default=None, description="Character encoding of the output file.")
    output_encoding_same_as_input: bool | None = Field(default=None, description="Use the input file's encoding for the output.")
    output_location: str | None = Field(default=None, description="Custom output location; replaces key_prefix and file_name (type output).")
    parquet_schema: ParquetSchemaInfo | None = Field(default=None, description="Schema of the parquet file; typically populated by JetStore from the input file.")
    put_headers_on_first_partition: bool | None = Field(default=None, description="Write the header line on the first partition file.")
    quote_all_records: bool | None = Field(default=None, description="Quote every record on the csv writer.")
    schema_provider: str | None = Field(default=None, description="Key of a schema_providers entry; alternative to format (types stage, output).")
    use_input_parquet_schema: bool | None = Field(default=None, description="Use the same parquet schema as the input file (types stage, output).")
    use_original_headers: bool | None = Field(default=None, description="Use the original headers that came with the input file (csv only) rather than the uniquefied headers JetStore processes with; applies to type output and to the stage channels leading to an output channel.")
    write_date_layout: str | None = Field(default=None, description="Date layout used to format dates on write.")
    write_step_id: str | None = Field(default=None, description="Step id in the stage location to write to (type stage).")


class OutputChannelConfigSql(OutputChannelConfigBase):
    """Configuration for a channel writing records to JetStore db (postgres)."""
    type: Literal["sql"] = Field(description="The channel type. Range: memory (default), stage, output, sql.")
    channel_spec_name: str | None = Field(default=None, description="Key of the channels entry providing this channel's spec, when it differs from name.")
    name: str | None = Field(default=None, description="Name: output channel name, required (must exist in the channels section of the config document) Format: file format, range values: csv, headerless_csv, fixed_width. NbrRowsInRecord: nbr of rows in record (applicable to format: parquet) Compression: none, snappy (default). Does not apply to parquet format (always snappy). UseInputParquetSchema to use the same schema as the input file. UseOriginalHeaders to use the headers from the input file (csv only). Must have save_parquet_schema = true in the cpipes first input_channel. OutputLocation: jetstore_s3_schema_events, jetstore_s3_input, jetstore_s3_output (default), or custom location. When OutputLocation is jetstore_s3_input it will also write to the input bucket. When using jetstore_s3_input and jetstore_s3_schema_events you must specify WriteStepId to specify the step id in stage location to output the file. When OutputLocation uses a custom location, it replaces KeyPrefix and FileName. OutputLocation must ends with \"/\" if we want to use default file name (i.e. OutputLocation does not include the file name). Engine default: computed (validator).")
    output_table_key: str = Field(description="Key of the output_tables entry to write to (type sql).")


class OutputFileSpec(_Base):
    """Shared configuration for pipeline output location."""
    bad_rows_config: BadRowsSpec | None = Field(default=None, description="How to identify and route bad input rows.")
    blank_field_markers: BlankFieldMarkersSpec | None = Field(default=None, description="Marker texts treated as blank (null) values.")
    bucket: str | None = Field(default=None, description="S3 bucket holding the file.")
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    compression: str | None = Field(default=None, description="File compression: none or snappy.")
    delimiter: int | None = Field(default=None, description="Field delimiter, as a code point (a json number).")
    detect_cr_as_eol: bool | None = Field(default=None, description="Detect whether \\r is used as eol (csv, headerless_csv).")
    detect_encoding: bool | None = Field(default=None, description="Detect the file encoding (limited), for text formats.")
    discard_file_headers: bool | None = Field(default=None, description="Discard the input file's headers; headers then come from the configuration or the schema provider.")
    domain_class: str | None = Field(default=None, description="Domain class of the records.")
    domain_keys: dict[str, str | list[str]] | None = Field(default=None, description="Domain key configuration for the records.")
    drop_excedent_headers: bool | None = Field(default=None, description="Drop headers beyond the configured columns.")
    encoding: str | None = Field(default=None, description="Character encoding of the file.")
    enforce_row_max_length: bool | None = Field(default=None, description="Fail rows with extra characters past the last field (text formats).")
    enforce_row_min_length: bool | None = Field(default=None, description="Require all columns in the input record; otherwise missing columns are null (text formats).")
    eol_byte: int | None = Field(default=None, description="Byte to use as eol (csv, headerless_csv).")
    fail_on_empty_column_name: bool | None = Field(default=None, description="Fail when a column name is empty (csv), preventing a data row from standing as headers.")
    file_key: str | None = Field(default=None, description="S3 object key of the file, or its directory when is_part_files; on output configs this is the output location.")
    file_name: str | None = Field(default=None, description="File name of the output file (type output).")
    fixed_width_columns_csv: str | None = Field(default=None, description="Column names and offsets for the fixed_width format, as csv text.")
    format: str | None = Field(default=None, description="File format: csv, headerless_csv, fixed_width, parquet, etc.")
    headers: list[str] | None = Field(default=None, description="Overrides the headers from the input channel's spec or the schema provider.")
    input_format_data_json: str | None = Field(default=None, description="Format-specific reader config as json, eg the sheet name for xlsx.")
    is_part_files: bool | None = Field(default=None, description="The file key is a directory of part files rather than a single file.")
    key: str = Field(description="Key referenced by a merge_files pipe's output_file.")
    key_prefix: str | None = Field(default=None, description="KeyPrefix is optional, default to input file key path in OutputLocation. Name is file name (required or via OutputLocation). Headers overrides the headers from the input_channel's spec or from the schema_provider. Schema provider indicates if put the header line or not. The input channel's schema provider indicates what delimiter to use on the header line.")
    lookback_periods: str | None = Field(default=None, description="Reads earlier source periods when the input is selected by explicit file_key from the stage area: the key is expanded once per period, substituting ${PERIOD_ID}, and the file listings are concatenated. Format: 'N' reads the current period plus N periods back; 'offset:last' reads current-offset back through current-last ('1:1' is the previous period only). Values may reference env vars; ${PERIOD_ID_TYPE} must name the current period id in the env.")
    main_input_row_count: int | None = Field(default=None, description="The total record count of the main input: the sum of input_records_count at step reducing00. start_reducing_cp carries it from step to step and exposes it to When expressions as $MAIN_INPUT_ROW_COUNT.")
    multi_columns_input: bool | None = Field(default=None, description="Require multiple columns in the input file, to detect a wrong delimiter (csv, headerless_csv).")
    name: str | None = Field(default=None, description="The output file name.")
    nbr_rows_in_record: int | None = Field(default=None, description="Nbr of rows in a record (format parquet).")
    no_quotes: bool | None = Field(default=None, description="Never quote records on the csv writer, even when a record contains a quote.")
    output_encoding: str | None = Field(default=None, description="Character encoding of the output file.")
    output_encoding_same_as_input: bool | None = Field(default=None, description="Use the input file's encoding for the output.")
    output_location: str | None = Field(default=None, description="The output location.")
    parquet_schema: ParquetSchemaInfo | None = Field(default=None, description="Schema of the parquet file; typically populated by JetStore from the input file.")
    put_headers_on_first_partition: bool | None = Field(default=None, description="Write the header line on the first partition file.")
    quote_all_records: bool | None = Field(default=None, description="Quote every record on the csv writer.")
    read_batch_size: int | None = Field(default=None, description="Nbr of rows to read per record (format parquet).")
    read_date_layout: str | None = Field(default=None, description="Date layout used to parse dates on read.")
    reorder_columns_on_read: list[int] | None = Field(default=None, description="Column positions used to reorder the input columns on read.")
    schema_provider: str | None = Field(default=None, description="Schema provider giving headers and the header-line delimiter.")
    trim_columns: bool | None = Field(default=None, description="Trim whitespace around column values.")
    use_lazy_quotes: bool | None = Field(default=None, description="Tolerant csv quote handling; see csv.NewReader.")
    use_lazy_quotes_special: bool | None = Field(default=None, description="Variant of use_lazy_quotes; see csv.NewReader.")
    use_original_headers: bool | None = Field(default=None, description="Use the original headers that came with the input file rather than the uniquefied headers JetStore processes with (csv).")
    variable_fields_per_record: bool | None = Field(default=None, description="Allow a variable number of fields per record; see csv.NewReader.")
    write_date_layout: str | None = Field(default=None, description="Date layout used to format dates on write.")


class ParquetSchemaInfo(_Base):
    """Define the schema of a parquet file"""
    fields: list[FieldInfo] | None = Field(default=None, description="The fields of the schema.")


class ParseDateFTSpec(_Base):
    """Identify dates that fall within a specified range, e.g. dob when > 1920 and < 2010."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    token: str = Field(description="The classification token this date test produces.")
    year_greater_than: int | None = Field(default=None, description="Additional match condition: year greater than this.")
    year_less_than: int | None = Field(default=None, description="Additional match condition: year less than this.")


class ParseDateSpec(_Base):
    """Configuration for parse_date function: date formats, null dates, etc."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    date_format_lookup: DateFormatLookupSpec | None = Field(default=None, description="DateFormatLookup: lookup table to use for date format parsing, with the following columns: - lookup_key: the key to match in the lookup table as int (column position in the channel) - lookup_values: the column name in the lookup table that contains the column's date format.")
    date_format_token: str | None = Field(default=None, description="DateFormatToken: output column name for listing up to 3 formats used in file.")
    date_formats: list[list[str]] | None = Field(default=None, description="DateFormats: list of date formats to use for parsing the date.")
    minmax_date_format: str | None = Field(default=None, description="MinMaxDateFormat: format used in output report for min/max dates.")
    null_dates: list[str] | None = Field(default=None, description="NullDates: list of date values to consider as null.")
    other_date_format_token: str | None = Field(default=None, description="OtherDateFormatToken: output column name to put the count of other format used in file.")
    other_date_formats: list[list[str]] | None = Field(default=None, description="OtherDateFormats: list of other date formats to use for parsing the date when DateFormatToken does not match (which are undesirable formats).")
    parse_date_args: list[ParseDateFTSpec] | None = Field(default=None, description="ParseDateArguments: list of parse date function token spec.")
    sampling_max_count: int | None = Field(default=None, description="DateSamplingMaxCount: nbr of samples to use for determining the date format.")
    use_jetstore_date_parser: bool | None = Field(default=None, description="UseJetstoreParser: when true it will use only the jetstore date parser. Identify top date format matches, up to 3. The first match must account for 75% of total date matches. Identify other date matches, each must match 98% of total date matches.")


class PartitionWriterSpec(_Base):
    """How the partition_writer operator writes its files."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    device_writer_type: Literal["csv_writer", "parquet_writer", "fixed_width_writer"] | None = Field(default=None, description="The writer to use for the partition files. Required when: absent(output_channel.schema_provider).")
    jets_partition_key: str | None = Field(default=None, description="Column carrying the jets partition key.")
    partition_size: int | None = Field(default=None, description="Rows per partition file.")
    sampling_max_count: int | None = Field(default=None, description="Cap on the sampled rows.")
    sampling_rate: int | None = Field(default=None, description="Sample one row in n.")
    stream_data_out: bool | None = Field(default=None, description="Stream the records out as they are written.")


class PipeSpecBase(_Base):
    """PipeSpec: A pipe applying one or more transformations to the records of one input channel."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")


class PipeSpecFanOut(PipeSpecBase):
    """A pipe applying one or more transformations to the records of one input channel."""
    type: Literal["fan_out"] = Field(description="The pipe type. Range: fan_out, splitter, merge_files.")
    apply: list[TransformationSpec] | None = Field(default=None, description="The transformations applied to each record, in order.")
    input_channel: InputChannelConfig = Field(description="The channel this pipe reads from.")


class PipeSpecMergeFiles(PipeSpecBase):
    """A pipe merging the partition files of a stage channel into one output file."""
    type: Literal["merge_files"] = Field(description="The pipe type. Range: fan_out, splitter, merge_files.")
    input_channel: InputChannelConfig = Field(description="The channel this pipe reads from.")
    merge_file_config: MergeFileSpec | None = Field(default=None, description="Configuration of the merge: whether the first partition carries the headers.")
    output_file: str = Field(description="Key of the output_files entry describing the merged file.")


class PipeSpecSplitter(PipeSpecBase):
    """A pipe splitting the records of one input channel into partitions, then applying its transformations to each partition."""
    type: Literal["splitter"] = Field(description="The pipe kind. Selects which of the pipe-level config fields apply.")
    apply: list[TransformationSpec] | None = Field(default=None, description="The transformations applied to the records of each partition.")
    input_channel: InputChannelConfig = Field(description="The channel this pipe reads its records from.")
    splitter_config: SplitterSpec = Field(description="How to split the input records into partitions.")


class PromptTemplateSpec(_Base):
    """Shared configuration of prompt templates."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    key: str = Field(description="Key is the name used by OllamaSpec.PromptTemplateName.")
    response_format: str | dict[str, Any] | None = Field(default=None, description="Default response format for operators using this template.")
    system_prompt: str | None = Field(default=None, description="Default system message for operators using this template.")
    template: str = Field(description="Template is the prompt text, see OllamaSpec for the placeholder syntax. SystemPrompt and ResponseFormat are defaults for the operator using this template, the operator's own settings take precedence when both are provided.")


class RegexNode(_Base):
    """Ratio of column's data matching a regex expression."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    name: str = Field(description="The classification token this regex identifies.")
    re: str | None = Field(default=None, description="The regex expression.")
    use_scrubbed_value: bool | None = Field(default=None, description="Match against the scrubbed value rather than the raw one.")


class ReportCmdSpec(_Base):
    """A report command run by the schema provider; s3_copy_file copies a file from s3 to s3, optionally gated by a when expression."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    s3_copy_file_config: S3CopyFileSpec = Field(description="Configuration for the s3_copy_file command.")
    type: str = Field(description="The report command type. Range: s3_copy_file, which copies an object within S3 using multipart copy. Commands run after the pipeline from the schema provider's report_cmds, subject to the optional when expression.")
    when: ExpressionNode | None = Field(default=None, description="Optional expression to determine if the command is to be executed.")


class S3CopyFileSpec(_Base):
    """Configuration of the s3_copy_file report command: source and destination bucket/key. Default worker_pool_size is calculated from the number of tasks."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    dest_bucket: str | None = Field(default=None, description="The S3 bucket the object is copied to.")
    dest_key: str = Field(description="The key the object is copied to.")
    src_bucket: str | None = Field(default=None, description="The S3 bucket holding the object to copy.")
    src_key: str = Field(description="The key of the object to copy.")
    worker_pool_size: int | None = Field(default=None, description="Default is calculated based on number of tasks. Engine default: computed (builder).")


class SchemaColumnSpec(_Base):
    """A column of a schema provider: name, and length/precision for fixed_width."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    length: int | None = Field(default=None, description="For fixed_width.")
    name: str = Field(description="The name of the column.")
    precision: int | None = Field(default=None, description="For fixed_width.")


class SchemaProviderSpecBase(_Base):
    """SchemaProviderSpec: Schema Provider configuration, this is use to provide runtime configuration to pipeline or as event to start pipelines."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    request_id: str | None = Field(default=None, description="RequestID is used for logging and tracking purpose. Contains properties to register FileKey with input_registry table:")


class SchemaProviderSpecDefault(SchemaProviderSpecBase):
    """Schema Provider configuration, this is use to provide runtime configuration to pipeline or as event to start pipelines."""
    type: Literal["default"] = Field(description="Type: pipeline_coordinator_map. RequestId: request_id for the pipeline coordinator map.")
    blank_field_markers: BlankFieldMarkersSpec | None = Field(default=None, description="BlankFieldMarkers: specify markers for blank fields (any format, typically for csv format) GetPartitionsSize: when true, get the size of the partitions from s3")
    bucket: str | None = Field(default=None, description="S3 bucket holding the file.")
    cap_dob_years: int | None = Field(default=None, description="CapDobYears: number of years to cap dob (date of birth) to today - for Anonymization.")
    client: str | None = Field(default=None, description="Client, Vendor, ObjectType, FileDate (does not apply to Jets_Loader).")
    columns: list[SchemaColumnSpec] | None = Field(default=None, description="Columns: may be omitted if fixed_width_columns_csv is provided or is a csv format")
    compression: Literal["none", "snappy"] | None = Field(default=None, description="Compression: none, snappy (parquet is always snappy).")
    delimiter: int | None = Field(default=None, description="Field delimiter, as a code point (a json number).")
    detect_cr_as_eol: bool | None = Field(default=None, description="DetectCrAsEol: Detect if \\r is used as eol (format: csv,headerless_csv).")
    detect_encoding: bool | None = Field(default=None, description="DetectEncoding: Detect file encoding (limited) for text file format.")
    discard_file_headers: bool | None = Field(default=None, description="DiscardFileHeaders: when true, discard the headers from the input file (typically for csv format), this will force to use Headers or Columns from the configuration, or from the schema provider if Headers and Columns are not provided.")
    domain_class: str | None = Field(default=None, description="Domain class of the records.")
    domain_keys: dict[str, str | list[str]] | None = Field(default=None, description="Domain key configuration for the records.")
    drop_excedent_headers: bool | None = Field(default=None, description="Drop headers beyond the configured columns.")
    encoding: str | None = Field(default=None, description="Character encoding of the file.")
    enforce_row_max_length: bool | None = Field(default=None, description="EnforceRowMaxLength: when true, no extra characters must exist past last field (applies to text format).")
    enforce_row_min_length: bool | None = Field(default=None, description="EnforceRowMinLength: when true, all columns must be in input record, otherwise missing columns are null.")
    env: dict[str, str | int | float | bool] | None = Field(default=None, description="Env vars made available to the pipeline.")
    eol_byte: int | None = Field(default=None, description="EolByte: Byte to use as eol (format: csv,headerless_csv).")
    fail_on_empty_column_name: bool | None = Field(default=None, description="FailOnEmptyColumnName: when true, fail if a column name is empty (format: csv,headerless_csv) - this is to prevent using a data row as headers.")
    file_date: str | None = Field(default=None, description="Registration property for the input_registry table.")
    file_key: str | None = Field(default=None, description="S3 object key of the file, or its directory when is_part_files; on output configs this is the output location.")
    file_name: str | None = Field(default=None, description="File name of the output file (type output).")
    file_size: int | None = Field(default=None, description="Size of the file in bytes.")
    fixed_width_columns_csv: str | None = Field(default=None, description="Column names and offsets for the fixed_width format, as csv text.")
    format: Literal["csv", "headerless_csv", "fixed_width", "parquet", "parquet_select", "xlsx", "headerless_xlsx"] | None = Field(default=None, description="Format: csv, headerless_csv, fixed_width, parquet, parquet_select, xlsx, headerless_xlsx")
    headers: list[str] | None = Field(default=None, description="Headers: alt to Columns, typically for csv format BlankFieldMarkers: specify markers for blank fields (any format, typically for csv format) GetPartitionsSize: when true, get the size of the partitions from s3")
    input_format_data_json: str | None = Field(default=None, description="InputFormatDataJson: json config based on Format (typically used for xlsx).")
    is_part_files: bool | None = Field(default=None, description="The file key is a directory of part files rather than a single file.")
    key: str = Field(description="Key is schema provider key for reference by compute pipes steps Format: csv, headerless_csv, fixed_width, parquet, parquet_select, xlsx, headerless_xlsx Compression: none, snappy (parquet is always snappy). DetectEncoding: Detect file encoding (limited) for text file format. DetectCrAsEol: Detect if \\r is used as eol (format: csv,headerless_csv). DiscardFileHeaders: when true, discard the headers from the input file (typically for csv format), this will force to use Headers or Columns from the configuration, or from the schema provider if Headers and Columns are not provided. EolByte: Byte to use as eol (format: csv,headerless_csv). FailOnEmptyColumnName: when true, fail if a column name is empty (format: csv,headerless_csv) - this is to prevent using a data row as headers. MultiColumnsInput: Indicate that input file must have multiple columns, this is used to detect if the wrong delimiter is used (csv,headerless_csv). ReadBatchSize: nbr of rows to read per record (format: parquet). NbrRowsInRecord: nbr of rows in record (format: parquet). InputFormatDataJson: json config based on Format (typically used for xlsx).")
    key_prefix: str | None = Field(default=None, description="Key prefix of the output location; defaults to the input file key path.")
    kms_key_arn: str | None = Field(default=None, description="KmsKey is kms key to use when writing output data. May be empty.")
    lookback_periods: str | None = Field(default=None, description="Reads earlier source periods when the input is selected by explicit file_key from the stage area: the key is expanded once per period, substituting ${PERIOD_ID}, and the file listings are concatenated. Format: 'N' reads the current period plus N periods back; 'offset:last' reads current-offset back through current-last ('1:1' is the previous period only). Values may reference env vars; ${PERIOD_ID_TYPE} must name the current period id in the env.")
    multi_columns_input: bool | None = Field(default=None, description="MultiColumnsInput: Indicate that input file must have multiple columns, this is used to detect if the wrong delimiter is used (csv,headerless_csv).")
    nbr_rows_in_record: int | None = Field(default=None, description="NbrRowsInRecord: nbr of rows in record (format: parquet).")
    no_quotes: bool | None = Field(default=None, description="NoQuotes will no quote any records for csv writer (even if the record contains '\"'). Bucket and FileKey are location and source object (fileKey may be directory if IsPartFiles is true)")
    notification_routing_overrides_json: str | None = Field(default=None, description="Overrides of the notification routing, as json.")
    notification_templates_overrides: dict[str, str] | None = Field(default=None, description="Overrides of the deployment's notification templates.")
    notify_api_gateway_override: Literal["no_notifications", "failure_only", "start_only", "completion_and_failure_only", "default"] | None = Field(default=None, description="NotifyApiGatewayOverride: values: no_notifications, failure_only, start_only, completion_and_failure_only, default (same as empty). NotificationTemplatesOverrides have the following keys to override the templates defined in the deployment environment var: CPIPES_START_NOTIFICATION_JSON, CPIPES_COMPLETED_NOTIFICATION_JSON, and CPIPES_FAILED_NOTIFICATION_JSON. Properties for type pipeline_coordinator_map:")
    object_type: str | None = Field(default=None, description="Registration property for the input_registry table.")
    output_encoding: str | None = Field(default=None, description="Character encoding of the output file.")
    output_encoding_same_as_input: bool | None = Field(default=None, description="Use the input file's encoding for the output.")
    parquet_schema: ParquetSchemaInfo | None = Field(default=None, description="Schema of the parquet file; typically populated by JetStore from the input file.")
    put_headers_on_first_partition: bool | None = Field(default=None, description="Write the header line on the first partition file.")
    quote_all_records: bool | None = Field(default=None, description="QuoteAllRecords will quote all records for csv writer.")
    read_batch_size: int | None = Field(default=None, description="ReadBatchSize: nbr of rows to read per record (format: parquet).")
    read_date_layout: str | None = Field(default=None, description="Date layout used to parse dates on read.")
    reorder_columns_on_read: list[int] | None = Field(default=None, description="Column positions used to reorder the input columns on read.")
    report_cmds: list[ReportCmdSpec] | None = Field(default=None, description="Commands for the run_report step.")
    schema_name: str | None = Field(default=None, description="Name identifying the schema of the input data. The main input's schema name is exposed to config expressions as $MAIN_SCHEMA_NAME.")
    set_all_dates_to_jan1: bool | None = Field(default=None, description="SetAllDatesToJan1: set all dates to January 1st of the date year - for Anonymization. UseLazyQuotes, UseLazyQuotesSpecial, VariableFieldsPerRecord: see csv.NewReader. QuoteAllRecords will quote all records for csv writer. NoQuotes will no quote any records for csv writer (even if the record contains '\"'). Bucket and FileKey are location and source object (fileKey may be directory if IsPartFiles is true)")
    set_dob_to_jan1: bool | None = Field(default=None, description="SetDobToJan1: set dob (date of birth) to January 1st of the date year - for Anonymization.")
    set_dod_to_jan1: bool | None = Field(default=None, description="SetDodToJan1: set dod (date of death) to January 1st of the date year - for Anonymization.")
    source_type: Literal["main_input", "merged_input", "historical_input"] | None = Field(default=None, description="The input source kind: main_input, merged_input or historical_input.")
    trim_columns: bool | None = Field(default=None, description="Trim whitespace around column values.")
    use_lazy_quotes: bool | None = Field(default=None, description="UseLazyQuotes, UseLazyQuotesSpecial, VariableFieldsPerRecord: see csv.NewReader.")
    use_lazy_quotes_special: bool | None = Field(default=None, description="Variant of use_lazy_quotes; see csv.NewReader.")
    use_origin_source_config: bool | None = Field(default=None, description="UseOriginSourceConfig: when true, use the source config from file_key components (client, org, object_type).")
    variable_fields_per_record: bool | None = Field(default=None, description="Allow a variable number of fields per record; see csv.NewReader.")
    vendor: str | None = Field(default=None, description="Registration property for the input_registry table.")
    write_date_layout: str | None = Field(default=None, description="Date layout used to format dates on write.")


class SchemaProviderSpecPipelineCoordinatorMap(SchemaProviderSpecBase):
    """Define specialized schema provider to start parallel pipelines with a post pipeline"""
    type: Literal["pipeline_coordinator_map"] = Field(description="Type: pipeline_coordinator_map. RequestId: request_id for the pipeline coordinator map.")
    coordinated_pipes_map: list[SchemaProviderSpec] = Field(description="CoordinatedPipesMap: list of schema_event_json.")
    post_map_event: SchemaProviderSpec | None = Field(default=None, description="PostMapEvent: schema event for post map pipeline.")


class ShufflingSpec(_Base):
    """Configuration for the shuffle transformation operator:number of records to retain to draw values from, max number of output records, column filtering, etc."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    filter_columns: FilterColumnSpec | None = Field(default=None, description="Which columns are retained, via an analysis lookup.")
    max_input_sample_size: int | None = Field(default=None, description="Nbr of input records retained to draw values from. Engine default: 1000 (builder).")
    output_sample_size: int | None = Field(default=None, description="Nbr of output records generated.")
    pad_short_rows_with_nulls: bool | None = Field(default=None, description="Pad short rows with nulls to the schema length.")


class SortSpec(_Base):
    """Specify the composite key for sorting rows: using the domain key of the input channel class or by column names."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    domain_key: str | None = Field(default=None, description="Sort on the domain key of this object type. Required when: absent(sort_by).")
    is_debug: bool | None = Field(default=None, description="Log trace info.")
    sort_by: list[str] | None = Field(default=None, description="Columns forming the sort key. Required when: absent(domain_key).")


class SourcesConfigSpec(_Base):
    """Carry-over configuration from table source_config: the main input and the inputs merged or injected via domain keys."""
    injected_inputs: list[InputSourceSpec] | None = Field(default=None, description="Inputs injected via domain keys.")
    main_input: InputSourceSpec | None = Field(default=None, description="The main input source: its original and uniquefied columns, domain class, and domain-keys spec. Populated by the start lambdas from source_config and the schema providers.")
    merged_inputs: list[InputSourceSpec] | None = Field(default=None, description="Inputs merged via domain keys.")


class SplitterSpecBase(_Base):
    """SplitterSpec: Creates one partition per distinct value of the split key."""
    column: str | None = Field(default=None, description="The input column whose value names the partition. Required when: absent(default_splitter_value) and absent(shard_on).")
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    default_splitter_value: str | None = Field(default=None, description="A fixed partition name, used when the records are not split on a column value. Required when: absent(column) and absent(shard_on).")
    shard_on: HashExpression | None = Field(default=None, description="Hash the record on the fly and partition on the hash. Required when: absent(column) and absent(default_splitter_value).")


class SplitterSpecStandard(SplitterSpecBase):
    """Creates one partition per distinct value of the split key."""
    type: Literal["standard"] = Field(default="standard", description="Which splitting strategy to use. Engine default: standard (builder).")


class SplitterSpecExtCount(SplitterSpecBase):
    """Creates one partition per distinct value of the split key, subdivided so that no partition holds more than partition_row_count rows."""
    type: Literal["ext_count"] = Field(description="Which splitting strategy to use. Engine default: standard (builder).")
    partition_row_count: int = Field(description="Maximum number of rows in each extended partition.")


class TableColumnSpec(_Base):
    """Column metadata for lookup_tables and output_tables."""
    as_array: bool | None = Field(default=None, description="The column holds an array.")
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    name: str = Field(description="Column name.")
    rdf_type: str | None = Field(default=None, description="The column's rdf type.")


class TableSpec(_Base):
    """Shared configuration to load the pipeline output in JetStore DB (postgres)"""
    channel_spec_name: str | None = Field(default=None, description="ChannelSpecName specify the channel spec. Column provides metadata info")
    check_schema_changed: bool | None = Field(default=None, description="Verify whether the table schema changed before the load.")
    columns: list[TableColumnSpec] | None = Field(default=None, description="Column metadata of the table.")
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    key: str = Field(description="Key is the table key for reference by compute pipes steps")
    name: str = Field(description="Name is the table name for reference by compute pipes steps, env var replacement used for table name (e.g., ${CLIENT}_${OBJECT_TYPE})")


class TargetColumnsLookupSpec(_Base):
    """Lookup table driving the clustering operator's target columns: the lookup name, its data classification column and the classification values of the two column groups."""
    column1_classification_values: list[str] = Field(description="The data classification values selecting which input columns play the column1 role in the correlation analysis.")
    column2_classification_values: list[str] = Field(description="The data classification values selecting which input columns play the column2 role in the correlation analysis.")
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    data_classification_column: str = Field(description="The lookup-table column holding the data classification value.")
    lookup_name: str = Field(description="The lookup table, keyed by column name, giving each input column's data classification.")


class TransformationColumnSpecBase(_Base):
    """TransformationColumnSpec: Set the output with the average of the input values (aggregate function)."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")


class TransformationColumnSpecAvrg(TransformationColumnSpecBase):
    """Set the output with the average of the input values (aggregate function)."""
    type: Literal["avrg"] = Field(description="The column operator. Range: select, multi_select, value, eval, map, hash, count, distinct_count, sum, min, max, avrg, case, map_reduce, lookup.")
    as_rdf_type: str | None = Field(default=None, description="Casts the value; applies to select, multi_select, value and the min/max/sum/avrg aggregates.")
    expr: str = Field(description="The input column (select, map, aggregates) or the literal value (value).")
    name: str = Field(description="The output column to set.")
    where: ExpressionNode | None = Field(default=None, description="Filter on the input values entering the aggregate.")


class TransformationColumnSpecCase(TransformationColumnSpecBase):
    """Set the output using a sql-like case operator"""
    type: Literal["case"] = Field(description="The column operator. Range: select, multi_select, value, eval, map, hash, count, distinct_count, sum, min, max, avrg, case, map_reduce, lookup.")
    case_expr: list[CaseExpression] = Field(description="The when-then legs of the case operator.")
    else_expr: list[TransformationColumnSpec] | None = Field(default=None, description="The column transformations applied when no case leg matches.")
    name: str | None = Field(default=None, description="The output column to set.")


class TransformationColumnSpecCount(TransformationColumnSpecBase):
    """Set the output with the count of non null input values (aggregate function)."""
    type: Literal["count"] = Field(description="The column operator. Range: select, multi_select, value, eval, map, hash, count, distinct_count, sum, min, max, avrg, case, map_reduce, lookup.")
    expr: str = Field(description="The input column (select, map, aggregates) or the literal value (value).")
    name: str = Field(description="The output column to set.")
    where: ExpressionNode | None = Field(default=None, description="Filter on the input values entering the aggregate.")


class TransformationColumnSpecDistinctCount(TransformationColumnSpecBase):
    """Set the output with the count of non null distinct input values (aggregate function)."""
    type: Literal["distinct_count"] = Field(description="The column operator. Range: select, multi_select, value, eval, map, hash, count, distinct_count, sum, min, max, avrg, case, map_reduce, lookup.")
    expr: str = Field(description="The input column (select, map, aggregates) or the literal value (value).")
    name: str = Field(description="The output column to set.")
    where: ExpressionNode | None = Field(default=None, description="Filter on the input values entering the aggregate.")


class TransformationColumnSpecEval(TransformationColumnSpecBase):
    """Set the output to the result of an expression."""
    type: Literal["eval"] = Field(description="The column operator. Range: select, multi_select, value, eval, map, hash, count, distinct_count, sum, min, max, avrg, case, map_reduce, lookup.")
    eval_expr: ExpressionNode = Field(description="The expression tree evaluated by the eval operator.")
    name: str = Field(description="The output column to set.")


class TransformationColumnSpecHash(TransformationColumnSpecBase):
    """Set the output to the result of the hash expression."""
    type: Literal["hash"] = Field(description="The column operator. Range: select, multi_select, value, eval, map, hash, count, distinct_count, sum, min, max, avrg, case, map_reduce, lookup.")
    hash_expr: HashExpression = Field(description="The hash configuration for the hash operator.")
    name: str = Field(description="The output column to set.")


class TransformationColumnSpecLookup(TransformationColumnSpecBase):
    """Set the output with a value from a lookup table"""
    type: Literal["lookup"] = Field(description="The column operator. Range: select, multi_select, value, eval, map, hash, count, distinct_count, sum, min, max, avrg, case, map_reduce, lookup.")
    key: list[LookupColumnSpec] | None = Field(default=None, description="How the lookup key is built from the input record.")
    lookup_name: str = Field(description="The lookup table to query.")
    max_env_var_substitution: int | None = Field(default=None, description="Rounds of env var substitution applied to expr containing '$'.")
    name: str | None = Field(default=None, description="The output column to set.")
    values: list[LookupColumnSpec] | None = Field(default=None, description="How the looked-up values are placed on the output record.")


class TransformationColumnSpecMap(TransformationColumnSpecBase):
    """Map the input value into the output, applying transformation and data type conversion."""
    type: Literal["map"] = Field(description="The column operator. Range: select, multi_select, value, eval, map, hash, count, distinct_count, sum, min, max, avrg, case, map_reduce, lookup.")
    expr: str = Field(description="The input column (select, map, aggregates) or the literal value (value).")
    map_expr: MapExpression | None = Field(default=None, description="Cleansing, type casting and default value for the map operator.")
    name: str = Field(description="The output column to set.")


class TransformationColumnSpecMapReduce(TransformationColumnSpecBase):
    """Shard the input values based on a column, apply reducing transformation to each shard then reduce all shards using reducing functions, set the output with the resulting value."""
    type: Literal["map_reduce"] = Field(description="The column operator. Range: select, multi_select, value, eval, map, hash, count, distinct_count, sum, min, max, avrg, case, map_reduce, lookup.")
    alternate_map_on: list[str] | None = Field(default=None, description="Fallback columns for map_on when its value is nil or empty.")
    apply_map: list[TransformationColumnSpec] | None = Field(default=None, description="The column transformations applied to each shard (map phase).")
    apply_reduce: list[TransformationColumnSpec] | None = Field(default=None, description="The reducing transformations applied across the shards (reduce phase).")
    map_on: str | None = Field(default=None, description="The column whose value shards the input of map_reduce.")
    name: str | None = Field(default=None, description="The output column to set.")


class TransformationColumnSpecMax(TransformationColumnSpecBase):
    """Set the output with the max of the non null input values (aggregate function)."""
    type: Literal["max"] = Field(description="The column operator. Range: select, multi_select, value, eval, map, hash, count, distinct_count, sum, min, max, avrg, case, map_reduce, lookup.")
    as_rdf_type: str | None = Field(default=None, description="Casts the value; applies to select, multi_select, value and the min/max/sum/avrg aggregates.")
    expr: str = Field(description="The input column (select, map, aggregates) or the literal value (value).")
    name: str = Field(description="The output column to set.")
    where: ExpressionNode | None = Field(default=None, description="Filter on the input values entering the aggregate.")


class TransformationColumnSpecMin(TransformationColumnSpecBase):
    """Set the output with the min of the non null input values (aggregate function)."""
    type: Literal["min"] = Field(description="The column operator. Range: select, multi_select, value, eval, map, hash, count, distinct_count, sum, min, max, avrg, case, map_reduce, lookup.")
    as_rdf_type: str | None = Field(default=None, description="Casts the value; applies to select, multi_select, value and the min/max/sum/avrg aggregates.")
    expr: str = Field(description="The input column (select, map, aggregates) or the literal value (value).")
    name: str = Field(description="The output column to set.")
    where: ExpressionNode | None = Field(default=None, description="Filter on the input values entering the aggregate.")


class TransformationColumnSpecMultiSelect(TransformationColumnSpecBase):
    """Set the output with an array of the values from specified input columns."""
    type: Literal["multi_select"] = Field(description="The column operator. Range: select, multi_select, value, eval, map, hash, count, distinct_count, sum, min, max, avrg, case, map_reduce, lookup.")
    as_rdf_type: str | None = Field(default=None, description="Casts the value; applies to select, multi_select, value and the min/max/sum/avrg aggregates.")
    expr_array: list[str] = Field(description="The input columns for multi_select.")
    max_env_var_substitution: int | None = Field(default=None, description="Rounds of env var substitution applied to expr containing '$'.")
    name: str = Field(description="The output column to set.")


class TransformationColumnSpecSelect(TransformationColumnSpecBase):
    """Set the output by selecting from an input column."""
    type: Literal["select"] = Field(description="The column operator. Range: select, multi_select, value, eval, map, hash, count, distinct_count, sum, min, max, avrg, case, map_reduce, lookup.")
    as_rdf_type: str | None = Field(default=None, description="Casts the value; applies to select, multi_select, value and the min/max/sum/avrg aggregates.")
    expr: str = Field(description="The input column (select, map, aggregates) or the literal value (value).")
    max_env_var_substitution: int | None = Field(default=None, description="Rounds of env var substitution applied to expr containing '$'.")
    name: str = Field(description="The output column to set.")


class TransformationColumnSpecSum(TransformationColumnSpecBase):
    """Set the output with the sum of the non null input values (aggregate function)."""
    type: Literal["sum"] = Field(description="The column operator. Range: select, multi_select, value, eval, map, hash, count, distinct_count, sum, min, max, avrg, case, map_reduce, lookup.")
    as_rdf_type: str | None = Field(default=None, description="Casts the value; applies to select, multi_select, value and the min/max/sum/avrg aggregates.")
    expr: str = Field(description="The input column (select, map, aggregates) or the literal value (value).")
    name: str = Field(description="The output column to set.")
    where: ExpressionNode | None = Field(default=None, description="Filter on the input values entering the aggregate.")


class TransformationColumnSpecValue(TransformationColumnSpecBase):
    """Set the output to a specified value."""
    type: Literal["value"] = Field(description="The column operator. Range: select, multi_select, value, eval, map, hash, count, distinct_count, sum, min, max, avrg, case, map_reduce, lookup.")
    as_rdf_type: str | None = Field(default=None, description="Casts the value; applies to select, multi_select, value and the min/max/sum/avrg aggregates.")
    expr: str = Field(description="The input column (select, map, aggregates) or the literal value (value).")
    max_env_var_substitution: int | None = Field(default=None, description="Rounds of env var substitution applied to expr containing '$'.")
    name: str = Field(description="The output column to set.")


class TransformationSpecBase(_Base):
    """TransformationSpec: Calls the infer server once per record and augments the record in place with values extracted from the model response."""
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    conditional_config: list[ConditionalTransformationSpec] | None = Field(default=None, description="Conditions that override fields of this operator or replace it altogether.")
    when: ExpressionNode | None = Field(default=None, description="Guard: the transformation is applied only when this evaluates true.")


class TransformationSpecOllama(TransformationSpecBase):
    """Calls the infer server once per record and augments the record in place with values extracted from the model response."""
    type: Literal["ollama"] = Field(description="The operator. Range: map_record, aggregate, analyze, high_freq, partition_writer, anonymize, distinct, shuffling, group_by, filter, sort, merge, jetrules, clustering, ollama, embed.")
    columns: list[TransformationColumnSpec] | None = Field(default=None, description="Column transformations; the ollama operator maps through output_mapping instead.")
    ollama_config: OllamaSpec = Field(description="Configuration of the ollama operator.")
    output_channel: OutputChannelConfig = Field(description="The channel the operator writes to.")


class TransformationSpecEmbed(TransformationSpecBase):
    """Calls the infer server's embeddings endpoint once per record and puts the resulting vector on the record."""
    type: Literal["embed"] = Field(description="The operator. Range: map_record, aggregate, analyze, high_freq, partition_writer, anonymize, distinct, shuffling, group_by, filter, sort, merge, jetrules, clustering, ollama, embed.")
    columns: list[TransformationColumnSpec] | None = Field(default=None, description="Column transformations; the ollama operator maps through output_mapping instead.")
    embed_config: EmbedSpec = Field(description="Configuration of the embed operator.")
    output_channel: OutputChannelConfig = Field(description="The channel the operator writes to.")


class TransformationSpecPartitionWriter(TransformationSpecBase):
    """Writes the records of its input channel to partition files."""
    type: Literal["partition_writer"] = Field(description="The operator. Range: map_record, aggregate, analyze, high_freq, partition_writer, anonymize, distinct, shuffling, group_by, filter, sort, merge, jetrules, clustering, ollama, embed.")
    columns: list[TransformationColumnSpec] | None = Field(default=None, description="Column transformations applied before the write.")
    new_record: bool | None = Field(default=None, description="Emit a new record rather than augmenting the input one.")
    output_channel: OutputChannelConfig = Field(description="The channel the operator writes to.")
    partition_writer_config: PartitionWriterSpec = Field(description="Configuration of the partition writer.")


class TransformationSpecMapRecord(TransformationSpecBase):
    """Maps the input record onto the output channel's columns."""
    type: Literal["map_record"] = Field(description="The operator. Range: map_record, aggregate, analyze, high_freq, partition_writer, anonymize, distinct, shuffling, group_by, filter, sort, merge, jetrules, clustering, ollama, embed.")
    columns: list[TransformationColumnSpec] | None = Field(default=None, description="The column transformations. The operator's payload.")
    map_record_config: MapRecordSpec | None = Field(default=None, description="Optional file_mapping table driving the mapping, with its error channel.")
    new_record: bool | None = Field(default=None, description="Emit a new record rather than augmenting the input one.")
    output_channel: OutputChannelConfig = Field(description="The channel the operator writes to.")


class TransformationSpecAggregate(TransformationSpecBase):
    """Aggregate the input records to a single output record"""
    type: Literal["aggregate"] = Field(description="The operator. Range: map_record, aggregate, analyze, high_freq, partition_writer, anonymize, distinct, shuffling, group_by, filter, sort, merge, jetrules, clustering, ollama, embed.")
    columns: list[TransformationColumnSpec] | None = Field(default=None, description="The column transformations producing the output record.")
    new_record: bool = Field(description="Emit a new record rather than augmenting the input one.")
    output_channel: OutputChannelConfig = Field(description="The channel the operator writes to.")


class TransformationSpecAnalyze(TransformationSpecBase):
    """Analyze each columns of the input records, emit a record per column with various metrics to the output channel"""
    type: Literal["analyze"] = Field(description="The operator. Range: map_record, aggregate, analyze, high_freq, partition_writer, anonymize, distinct, shuffling, group_by, filter, sort, merge, jetrules, clustering, ollama, embed.")
    analyze_config: AnalyzeSpec = Field(description="Configuration of the analyze operator.")
    columns: list[TransformationColumnSpec] | None = Field(default=None, description="The column transformations producing the output record.")
    new_record: bool | None = Field(default=None, description="Emit a new record rather than augmenting the input one.")
    output_channel: OutputChannelConfig = Field(description="The channel the operator writes to.")


class TransformationSpecHighFreq(TransformationSpecBase):
    """Analyze selected column and send to output channel their top percentile values"""
    type: Literal["high_freq"] = Field(description="The operator. Range: map_record, aggregate, analyze, high_freq, partition_writer, anonymize, distinct, shuffling, group_by, filter, sort, merge, jetrules, clustering, ollama, embed.")
    columns: list[TransformationColumnSpec] | None = Field(default=None, description="The column transformations producing the output record.")
    high_freq_columns: list[HighFreqSpec] = Field(description="Configuration of the high_freq operator: the columns to analyze.")
    new_record: bool | None = Field(default=None, description="Emit a new record rather than augmenting the input one.")
    output_channel: OutputChannelConfig = Field(description="The channel the operator writes to.")


class TransformationSpecAnonymize(TransformationSpecBase):
    """Anonymize or de-identify input records using an analysis lookup identifying columns containing sensitive data."""
    type: Literal["anonymize"] = Field(description="The operator. Range: map_record, aggregate, analyze, high_freq, partition_writer, anonymize, distinct, shuffling, group_by, filter, sort, merge, jetrules, clustering, ollama, embed.")
    anonymize_config: AnonymizeSpec = Field(description="Configuration of the anonymize operator.")
    columns: list[TransformationColumnSpec] | None = Field(default=None, description="The column transformations producing the output record.")
    output_channel: OutputChannelConfig = Field(description="The channel the operator writes to.")


class TransformationSpecDistinct(TransformationSpecBase):
    """Filter the input records to retain a single record per composite key value, records sent to the output channel have a distinct composite key."""
    type: Literal["distinct"] = Field(description="The operator. Range: map_record, aggregate, analyze, high_freq, partition_writer, anonymize, distinct, shuffling, group_by, filter, sort, merge, jetrules, clustering, ollama, embed.")
    distinct_config: DistinctSpec = Field(description="Configuration of the distinct operator: the composite key.")
    output_channel: OutputChannelConfig = Field(description="The channel the operator writes to.")


class TransformationSpecShuffling(TransformationSpecBase):
    """Shuffle data on a per-column basis for retained columns specified in a lookup. The number of input records is capped by configuration. The number of generated output records is also specified by configuration."""
    type: Literal["shuffling"] = Field(description="The operator. Range: map_record, aggregate, analyze, high_freq, partition_writer, anonymize, distinct, shuffling, group_by, filter, sort, merge, jetrules, clustering, ollama, embed.")
    output_channel: OutputChannelConfig = Field(description="The channel the operator writes to.")
    shuffling_config: ShufflingSpec = Field(description="Configuration of the shuffling operator.")


class TransformationSpecGroupBy(TransformationSpecBase):
    """Group the input records into bundles using a composite key. Each output records correspond to a bundle where each element of the output record contains an input record."""
    type: Literal["group_by"] = Field(description="The operator. Range: map_record, aggregate, analyze, high_freq, partition_writer, anonymize, distinct, shuffling, group_by, filter, sort, merge, jetrules, clustering, ollama, embed.")
    group_by_config: GroupBySpec = Field(description="Configuration of the group_by operator.")
    output_channel: OutputChannelConfig = Field(description="The channel the operator writes to.")


class TransformationSpecFilter(TransformationSpecBase):
    """Apply a filter to input records, sending retained records to the output channel. Cap the number of retained records if specified."""
    type: Literal["filter"] = Field(description="The operator. Range: map_record, aggregate, analyze, high_freq, partition_writer, anonymize, distinct, shuffling, group_by, filter, sort, merge, jetrules, clustering, ollama, embed.")
    columns: list[TransformationColumnSpec] | None = Field(default=None, description="The column transformations producing the output record.")
    filter_config: FilterSpec | None = Field(default=None, description="Configuration of the filter operator.")
    new_record: bool | None = Field(default=None, description="Emit a new record rather than augmenting the input one.")
    output_channel: OutputChannelConfig = Field(description="The channel the operator writes to.")


class TransformationSpecSort(TransformationSpecBase):
    """Sort the input records in memory, send the sorted records to output channel."""
    type: Literal["sort"] = Field(description="The operator. Range: map_record, aggregate, analyze, high_freq, partition_writer, anonymize, distinct, shuffling, group_by, filter, sort, merge, jetrules, clustering, ollama, embed.")
    output_channel: OutputChannelConfig = Field(description="The channel the operator writes to.")
    sort_config: SortSpec = Field(description="Configuration of the sort operator.")


class TransformationSpecMerge(TransformationSpecBase):
    """Merge sorted records from multiple channels using a grouping composite key. All input channels must be sorted using the same logical sort order. Bundled records are sent to the output channel, each element of the output records correspond to an input record."""
    type: Literal["merge"] = Field(description="The operator. Range: map_record, aggregate, analyze, high_freq, partition_writer, anonymize, distinct, shuffling, group_by, filter, sort, merge, jetrules, clustering, ollama, embed.")
    merge_config: MergeSpec = Field(description="Configuration of the merge operator: the channels to merge and how to group them.")
    output_channel: OutputChannelConfig = Field(description="The channel the operator writes to.")


class TransformationSpecJetrules(TransformationSpecBase):
    """Input channel contains either individual or bundled input records. Each record or bundle are sent to JetRules as a rule session. Output records are extracted by type from the rule session, each type correspond to an output channel."""
    type: Literal["jetrules"] = Field(description="The operator. Range: map_record, aggregate, analyze, high_freq, partition_writer, anonymize, distinct, shuffling, group_by, filter, sort, merge, jetrules, clustering, ollama, embed.")
    columns: list[TransformationColumnSpec] | None = Field(default=None, description="The column transformations producing the output record.")
    jetrules_config: JetrulesSpec = Field(description="Configuration of the jetrules operator.")
    new_record: bool | None = Field(default=None, description="Emit a new record rather than augmenting the input one.")


class TransformationSpecClustering(TransformationSpecBase):
    """Compute the correlation between sets of columns identified by classification via a lookup table."""
    type: Literal["clustering"] = Field(description="The operator. Range: map_record, aggregate, analyze, high_freq, partition_writer, anonymize, distinct, shuffling, group_by, filter, sort, merge, jetrules, clustering, ollama, embed.")
    clustering_config: ClusteringSpec = Field(description="Configuration of the clustering operator.")
    new_record: bool | None = Field(default=None, description="Emit a new record rather than augmenting the input one.")
    output_channel: OutputChannelConfig = Field(description="The channel the operator writes to.")


class TransformationSpecOverride(_Base):
    """Fragments of transformation operator to override fields of the host transformation operator."""
    analyze_config: AnalyzeSpec | None = Field(default=None, description="Configuration of the analyze operator.")
    anonymize_config: AnonymizeSpec | None = Field(default=None, description="Configuration of the anonymize operator.")
    clustering_config: ClusteringSpec | None = Field(default=None, description="Configuration of the clustering operator.")
    columns: list[TransformationColumnSpec] | None = Field(default=None, description="The column transformations producing the output record.")
    comment: str | None = Field(default=None, description="Free text for the reader; ignored by JetStore.")
    distinct_config: DistinctSpec | None = Field(default=None, description="Configuration of the distinct operator: the composite key.")
    filter_config: FilterSpec | None = Field(default=None, description="Configuration of the filter operator.")
    group_by_config: GroupBySpec | None = Field(default=None, description="Configuration of the group_by operator.")
    high_freq_columns: list[HighFreqSpec] | None = Field(default=None, description="Configuration of the high_freq operator: the columns to analyze.")
    jetrules_config: JetrulesSpec | None = Field(default=None, description="Configuration of the jetrules operator.")
    map_record_config: MapRecordSpec | None = Field(default=None, description="Configuration of the map_record operator.")
    output_channel: OutputChannelConfig | None = Field(default=None, description="The channel the operator writes to.")
    partition_writer_config: PartitionWriterSpec | None = Field(default=None, description="Configuration of the partition writer.")
    shuffling_config: ShufflingSpec | None = Field(default=None, description="Configuration of the shuffling operator.")
    sort_config: SortSpec | None = Field(default=None, description="Configuration of the sort operator.")



AnonymizeSpec = Annotated[Union[AnonymizeSpecAnonymization, AnonymizeSpecDeIdentification], Field(discriminator="mode"), BeforeValidator(_tag_default("mode", "anonymization"))]
CsvSourceSpec = Annotated[Union[CsvSourceSpecCpipes, CsvSourceSpecCsvFile], Field(discriminator="type")]
FunctionTokenNode = Annotated[Union[FunctionTokenNodeParseDate, FunctionTokenNodeParseDouble, FunctionTokenNodeParseText], Field(discriminator="type")]
InputChannelConfig = Annotated[Union[InputChannelConfigMemory, InputChannelConfigInput, InputChannelConfigStage, InputChannelConfigGenerator], Field(discriminator="type"), BeforeValidator(_tag_default("type", "memory"))]
LookupColumnSpec = Annotated[Union[LookupColumnSpecSelect, LookupColumnSpecValue], Field(discriminator="type")]
LookupSpec = Annotated[Union[LookupSpecSqlLookup, LookupSpecS3CsvLookup], Field(discriminator="type")]
OutputChannelConfig = Annotated[Union[OutputChannelConfigMemory, OutputChannelConfigStage, OutputChannelConfigOutput, OutputChannelConfigSql], Field(discriminator="type"), BeforeValidator(_tag_default("type", "memory"))]
PipeSpec = Annotated[Union[PipeSpecFanOut, PipeSpecMergeFiles, PipeSpecSplitter], Field(discriminator="type")]
SchemaProviderSpec = Annotated[Union[SchemaProviderSpecDefault, SchemaProviderSpecPipelineCoordinatorMap], Field(discriminator="type")]
SplitterSpec = Annotated[Union[SplitterSpecStandard, SplitterSpecExtCount], Field(discriminator="type"), BeforeValidator(_tag_default("type", "standard"))]
TransformationColumnSpec = Annotated[Union[TransformationColumnSpecAvrg, TransformationColumnSpecCase, TransformationColumnSpecCount, TransformationColumnSpecDistinctCount, TransformationColumnSpecEval, TransformationColumnSpecHash, TransformationColumnSpecLookup, TransformationColumnSpecMap, TransformationColumnSpecMapReduce, TransformationColumnSpecMax, TransformationColumnSpecMin, TransformationColumnSpecMultiSelect, TransformationColumnSpecSelect, TransformationColumnSpecSum, TransformationColumnSpecValue], Field(discriminator="type")]
TransformationSpec = Annotated[Union[TransformationSpecOllama, TransformationSpecEmbed, TransformationSpecPartitionWriter, TransformationSpecMapRecord, TransformationSpecAggregate, TransformationSpecAnalyze, TransformationSpecHighFreq, TransformationSpecAnonymize, TransformationSpecDistinct, TransformationSpecShuffling, TransformationSpecGroupBy, TransformationSpecFilter, TransformationSpecSort, TransformationSpecMerge, TransformationSpecJetrules, TransformationSpecClustering], Field(discriminator="type")]


# class -> (go_struct, type_token); the reflect direction's key.
_MATRIX_KEYS = {
    "AnalyzeSpec": ("AnalyzeSpec", "*"),
    "AnonymizeSpecAnonymization": ("AnonymizeSpec", "anonymization"),
    "AnonymizeSpecDeIdentification": ("AnonymizeSpec", "de-identification"),
    "BadRowsSpec": ("BadRowsSpec", "*"),
    "BlankFieldMarkersSpec": ("BlankFieldMarkersSpec", "*"),
    "CaseEnvExpression": ("CaseEnvExpression", "*"),
    "CaseExpression": ("CaseExpression", "*"),
    "ChannelSpec": ("ChannelSpec", "*"),
    "ClusterShardingInfo": ("ClusterShardingInfo", "*"),
    "ClusterShardingSpec": ("ClusterShardingSpec", "*"),
    "ClusterSpec": ("ClusterSpec", "*"),
    "ClusteringSpec": ("ClusteringSpec", "*"),
    "ColumnEncodingSpec": ("ColumnEncodingSpec", "*"),
    "ColumnFileSpec": ("ColumnFileSpec", "*"),
    "ColumnNameLookupNode": ("ColumnNameLookupNode", "*"),
    "ColumnNameTokenNode": ("ColumnNameTokenNode", "*"),
    "ComputePipesCommonArgs": ("ComputePipesCommonArgs", "*"),
    "ComputePipesConfig": ("ComputePipesConfig", "*"),
    "ConditionalEnvVariable": ("ConditionalEnvVariable", "*"),
    "ConditionalPipeSpec": ("ConditionalPipeSpec", "*"),
    "ConditionalTransformationSpec": ("ConditionalTransformationSpec", "*"),
    "CsvSourceSpecCpipes": ("CsvSourceSpec", "cpipes"),
    "CsvSourceSpecCsvFile": ("CsvSourceSpec", "csv_file"),
    "DateFormatLookupSpec": ("DateFormatLookupSpec", "*"),
    "DistinctSpec": ("DistinctSpec", "*"),
    "DomainKeyInfo": ("DomainKeyInfo", "*"),
    "DomainKeysSpec": ("DomainKeysSpec", "*"),
    "EmbedSpec": ("EmbedSpec", "*"),
    "EntityHint": ("EntityHint", "*"),
    "FieldInfo": ("FieldInfo", "*"),
    "FilterColumnSpec": ("FilterColumnSpec", "*"),
    "FilterSpec": ("FilterSpec", "*"),
    "FunctionTokenNodeParseDate": ("FunctionTokenNode", "parse_date"),
    "FunctionTokenNodeParseDouble": ("FunctionTokenNode", "parse_double"),
    "FunctionTokenNodeParseText": ("FunctionTokenNode", "parse_text"),
    "GroupBySpec": ("GroupBySpec", "*"),
    "HashExpression": ("HashExpression", "*"),
    "HighFreqSpec": ("HighFreqSpec", "*"),
    "InferMappingSpec": ("InferMappingSpec", "*"),
    "InputChannelConfigGenerator": ("InputChannelConfig", "generator"),
    "InputChannelConfigInput": ("InputChannelConfig", "input"),
    "InputChannelConfigMemory": ("InputChannelConfig", "memory"),
    "InputChannelConfigStage": ("InputChannelConfig", "stage"),
    "InputSourceSpec": ("InputSourceSpec", "*"),
    "JetrulesSpec": ("JetrulesSpec", "*"),
    "KeywordTokenNode": ("KeywordTokenNode", "*"),
    "LookupColumnSpecSelect": ("LookupColumnSpec", "select"),
    "LookupColumnSpecValue": ("LookupColumnSpec", "value"),
    "LookupSpecS3CsvLookup": ("LookupSpec", "s3_csv_lookup"),
    "LookupSpecSqlLookup": ("LookupSpec", "sql_lookup"),
    "LookupTokenNode": ("LookupTokenNode", "*"),
    "MapExpression": ("MapExpression", "*"),
    "MapRecordSpec": ("MapRecordSpec", "*"),
    "MergeFileSpec": ("MergeFileSpec", "*"),
    "MergeSpec": ("MergeSpec", "*"),
    "Metric": ("Metric", "*"),
    "MetricsSpec": ("MetricsSpec", "*"),
    "MultiTokensNode": ("MultiTokensNode", "*"),
    "OllamaServerSpec": ("OllamaServerSpec", "*"),
    "OllamaSpec": ("OllamaSpec", "*"),
    "OutputChannelConfigMemory": ("OutputChannelConfig", "memory"),
    "OutputChannelConfigOutput": ("OutputChannelConfig", "output"),
    "OutputChannelConfigSql": ("OutputChannelConfig", "sql"),
    "OutputChannelConfigStage": ("OutputChannelConfig", "stage"),
    "OutputFileSpec": ("OutputFileSpec", "*"),
    "ParquetSchemaInfo": ("ParquetSchemaInfo", "*"),
    "ParseDateFTSpec": ("ParseDateFTSpec", "*"),
    "ParseDateSpec": ("ParseDateSpec", "*"),
    "PartitionWriterSpec": ("PartitionWriterSpec", "*"),
    "PipeSpecFanOut": ("PipeSpec", "fan_out"),
    "PipeSpecMergeFiles": ("PipeSpec", "merge_files"),
    "PipeSpecSplitter": ("PipeSpec", "splitter"),
    "PromptTemplateSpec": ("PromptTemplateSpec", "*"),
    "RegexNode": ("RegexNode", "*"),
    "ReportCmdSpec": ("ReportCmdSpec", "s3_copy_file"),
    "S3CopyFileSpec": ("S3CopyFileSpec", "*"),
    "SchemaColumnSpec": ("SchemaColumnSpec", "*"),
    "SchemaProviderSpecDefault": ("SchemaProviderSpec", "default"),
    "SchemaProviderSpecPipelineCoordinatorMap": ("SchemaProviderSpec", "pipeline_coordinator_map"),
    "ShufflingSpec": ("ShufflingSpec", "*"),
    "SortSpec": ("SortSpec", "*"),
    "SourcesConfigSpec": ("SourcesConfigSpec", "*"),
    "SplitterSpecExtCount": ("SplitterSpec", "ext_count"),
    "SplitterSpecStandard": ("SplitterSpec", "standard"),
    "TableColumnSpec": ("TableColumnSpec", "*"),
    "TableSpec": ("TableSpec", "*"),
    "TargetColumnsLookupSpec": ("TargetColumnsLookupSpec", "*"),
    "TransformationColumnSpecAvrg": ("TransformationColumnSpec", "avrg"),
    "TransformationColumnSpecCase": ("TransformationColumnSpec", "case"),
    "TransformationColumnSpecCount": ("TransformationColumnSpec", "count"),
    "TransformationColumnSpecDistinctCount": ("TransformationColumnSpec", "distinct_count"),
    "TransformationColumnSpecEval": ("TransformationColumnSpec", "eval"),
    "TransformationColumnSpecHash": ("TransformationColumnSpec", "hash"),
    "TransformationColumnSpecLookup": ("TransformationColumnSpec", "lookup"),
    "TransformationColumnSpecMap": ("TransformationColumnSpec", "map"),
    "TransformationColumnSpecMapReduce": ("TransformationColumnSpec", "map_reduce"),
    "TransformationColumnSpecMax": ("TransformationColumnSpec", "max"),
    "TransformationColumnSpecMin": ("TransformationColumnSpec", "min"),
    "TransformationColumnSpecMultiSelect": ("TransformationColumnSpec", "multi_select"),
    "TransformationColumnSpecSelect": ("TransformationColumnSpec", "select"),
    "TransformationColumnSpecSum": ("TransformationColumnSpec", "sum"),
    "TransformationColumnSpecValue": ("TransformationColumnSpec", "value"),
    "TransformationSpecAggregate": ("TransformationSpec", "aggregate"),
    "TransformationSpecAnalyze": ("TransformationSpec", "analyze"),
    "TransformationSpecAnonymize": ("TransformationSpec", "anonymize"),
    "TransformationSpecClustering": ("TransformationSpec", "clustering"),
    "TransformationSpecDistinct": ("TransformationSpec", "distinct"),
    "TransformationSpecEmbed": ("TransformationSpec", "embed"),
    "TransformationSpecFilter": ("TransformationSpec", "filter"),
    "TransformationSpecGroupBy": ("TransformationSpec", "group_by"),
    "TransformationSpecHighFreq": ("TransformationSpec", "high_freq"),
    "TransformationSpecJetrules": ("TransformationSpec", "jetrules"),
    "TransformationSpecMapRecord": ("TransformationSpec", "map_record"),
    "TransformationSpecMerge": ("TransformationSpec", "merge"),
    "TransformationSpecOllama": ("TransformationSpec", "ollama"),
    "TransformationSpecOverride": ("TransformationSpec", "~override"),
    "TransformationSpecPartitionWriter": ("TransformationSpec", "partition_writer"),
    "TransformationSpecShuffling": ("TransformationSpec", "shuffling"),
    "TransformationSpecSort": ("TransformationSpec", "sort"),
}

_MODELS = [v for v in list(globals().values()) if isinstance(v, type) and issubclass(v, BaseModel) and v is not BaseModel]
for _m in _MODELS:
    _m.model_rebuild()
