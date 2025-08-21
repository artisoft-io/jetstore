package main

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsquicksight"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

type QuickSightTopicStackProps struct {
	awscdk.StackProps
}

func NewQuickSightTopicStack(scope constructs.Construct, id string, props *QuickSightTopicStackProps) awscdk.Stack {
	stack := awscdk.NewStack(scope, &id, &props.StackProps)

	// The code below shows an example of how to instantiate this type.

	cfnTopic := awsquicksight.NewCfnTopic(stack, jsii.String("MyCfnTopic"), &awsquicksight.CfnTopicProps{
		AwsAccountId: jsii.String("awsAccountId"),
		ConfigOptions: &awsquicksight.CfnTopic_TopicConfigOptionsProperty{
			QBusinessInsightsEnabled: jsii.Bool(true),
		},
		DataSets: []interface{}{
			&awsquicksight.CfnTopic_DatasetMetadataProperty{
				DatasetArn: jsii.String("datasetArn"),

				// the properties below are optional
				CalculatedFields: []interface{}{
					&awsquicksight.CfnTopic_TopicCalculatedFieldProperty{
						CalculatedFieldName: jsii.String("calculatedFieldName"),
						Expression:          jsii.String("expression"),

						// the properties below are optional
						Aggregation:                jsii.String("aggregation"),
						AllowedAggregations:        jsii.Strings("allowedAggregations"),
						CalculatedFieldDescription: jsii.String("calculatedFieldDescription"),
						CalculatedFieldSynonyms:    jsii.Strings("calculatedFieldSynonyms"),
						CellValueSynonyms: []interface{}{
							&awsquicksight.CfnTopic_CellValueSynonymProperty{
								CellValue: jsii.String("cellValue"),
								Synonyms:  jsii.Strings("synonyms"),
							},
						},
						ColumnDataRole: jsii.String("columnDataRole"),
						ComparativeOrder: &awsquicksight.CfnTopic_ComparativeOrderProperty{
							SpecifedOrder:                 jsii.Strings("specifedOrder"),
							TreatUndefinedSpecifiedValues: jsii.String("treatUndefinedSpecifiedValues"),
							UseOrdering:                   jsii.String("useOrdering"),
						},
						DefaultFormatting: &awsquicksight.CfnTopic_DefaultFormattingProperty{
							DisplayFormat: jsii.String("displayFormat"),
							DisplayFormatOptions: &awsquicksight.CfnTopic_DisplayFormatOptionsProperty{
								BlankCellFormat:   jsii.String("blankCellFormat"),
								CurrencySymbol:    jsii.String("currencySymbol"),
								DateFormat:        jsii.String("dateFormat"),
								DecimalSeparator:  jsii.String("decimalSeparator"),
								FractionDigits:    jsii.Number(123),
								GroupingSeparator: jsii.String("groupingSeparator"),
								NegativeFormat: &awsquicksight.CfnTopic_NegativeFormatProperty{
									Prefix: jsii.String("prefix"),
									Suffix: jsii.String("suffix"),
								},
								Prefix:             jsii.String("prefix"),
								Suffix:             jsii.String("suffix"),
								UnitScaler:         jsii.String("unitScaler"),
								UseBlankCellFormat: jsii.Bool(false),
								UseGrouping:        jsii.Bool(false),
							},
						},
						DisableIndexing:        jsii.Bool(false),
						IsIncludedInTopic:      jsii.Bool(false),
						NeverAggregateInFilter: jsii.Bool(false),
						NonAdditive:            jsii.Bool(false),
						NotAllowedAggregations: jsii.Strings("notAllowedAggregations"),
						SemanticType: &awsquicksight.CfnTopic_SemanticTypeProperty{
							FalseyCellValue: jsii.String("falseyCellValue"),
							FalseyCellValueSynonyms: &[]*string{
								jsii.String("falseyCellValueSynonyms"),
							},
							SubTypeName:     jsii.String("subTypeName"),
							TruthyCellValue: jsii.String("truthyCellValue"),
							TruthyCellValueSynonyms: &[]*string{
								jsii.String("truthyCellValueSynonyms"),
							},
							TypeName: jsii.String("typeName"),
							TypeParameters: map[string]*string{
								"typeParametersKey": jsii.String("typeParameters"),
							},
						},
						TimeGranularity: jsii.String("timeGranularity"),
					},
				},
				Columns: []interface{}{
					&awsquicksight.CfnTopic_TopicColumnProperty{
						ColumnName: jsii.String("columnName"),

						// the properties below are optional
						Aggregation: jsii.String("aggregation"),
						AllowedAggregations: &[]*string{
							jsii.String("allowedAggregations"),
						},
						CellValueSynonyms: []interface{}{
							&awsquicksight.CfnTopic_CellValueSynonymProperty{
								CellValue: jsii.String("cellValue"),
								Synonyms: &[]*string{
									jsii.String("synonyms"),
								},
							},
						},
						ColumnDataRole:     jsii.String("columnDataRole"),
						ColumnDescription:  jsii.String("columnDescription"),
						ColumnFriendlyName: jsii.String("columnFriendlyName"),
						ColumnSynonyms: &[]*string{
							jsii.String("columnSynonyms"),
						},
						ComparativeOrder: &awsquicksight.CfnTopic_ComparativeOrderProperty{
							SpecifedOrder: &[]*string{
								jsii.String("specifedOrder"),
							},
							TreatUndefinedSpecifiedValues: jsii.String("treatUndefinedSpecifiedValues"),
							UseOrdering:                   jsii.String("useOrdering"),
						},
						DefaultFormatting: &awsquicksight.CfnTopic_DefaultFormattingProperty{
							DisplayFormat: jsii.String("displayFormat"),
							DisplayFormatOptions: &awsquicksight.CfnTopic_DisplayFormatOptionsProperty{
								BlankCellFormat:   jsii.String("blankCellFormat"),
								CurrencySymbol:    jsii.String("currencySymbol"),
								DateFormat:        jsii.String("dateFormat"),
								DecimalSeparator:  jsii.String("decimalSeparator"),
								FractionDigits:    jsii.Number(123),
								GroupingSeparator: jsii.String("groupingSeparator"),
								NegativeFormat: &awsquicksight.CfnTopic_NegativeFormatProperty{
									Prefix: jsii.String("prefix"),
									Suffix: jsii.String("suffix"),
								},
								Prefix:             jsii.String("prefix"),
								Suffix:             jsii.String("suffix"),
								UnitScaler:         jsii.String("unitScaler"),
								UseBlankCellFormat: jsii.Bool(false),
								UseGrouping:        jsii.Bool(false),
							},
						},
						DisableIndexing:        jsii.Bool(false),
						IsIncludedInTopic:      jsii.Bool(false),
						NeverAggregateInFilter: jsii.Bool(false),
						NonAdditive:            jsii.Bool(false),
						NotAllowedAggregations: &[]*string{
							jsii.String("notAllowedAggregations"),
						},
						SemanticType: &awsquicksight.CfnTopic_SemanticTypeProperty{
							FalseyCellValue: jsii.String("falseyCellValue"),
							FalseyCellValueSynonyms: &[]*string{
								jsii.String("falseyCellValueSynonyms"),
							},
							SubTypeName:     jsii.String("subTypeName"),
							TruthyCellValue: jsii.String("truthyCellValue"),
							TruthyCellValueSynonyms: &[]*string{
								jsii.String("truthyCellValueSynonyms"),
							},
							TypeName: jsii.String("typeName"),
							TypeParameters: map[string]*string{
								"typeParametersKey": jsii.String("typeParameters"),
							},
						},
						TimeGranularity: jsii.String("timeGranularity"),
					},
				},
				DataAggregation: &awsquicksight.CfnTopic_DataAggregationProperty{
					DatasetRowDateGranularity: jsii.String("datasetRowDateGranularity"),
					DefaultDateColumnName:     jsii.String("defaultDateColumnName"),
				},
				DatasetDescription: jsii.String("datasetDescription"),
				DatasetName:        jsii.String("datasetName"),
				Filters: []interface{}{
					&awsquicksight.CfnTopic_TopicFilterProperty{
						FilterName:       jsii.String("filterName"),
						OperandFieldName: jsii.String("operandFieldName"),

						// the properties below are optional
						CategoryFilter: &awsquicksight.CfnTopic_TopicCategoryFilterProperty{
							CategoryFilterFunction: jsii.String("categoryFilterFunction"),
							CategoryFilterType:     jsii.String("categoryFilterType"),
							Constant: &awsquicksight.CfnTopic_TopicCategoryFilterConstantProperty{
								CollectiveConstant: &awsquicksight.CfnTopic_CollectiveConstantProperty{
									ValueList: &[]*string{
										jsii.String("valueList"),
									},
								},
								ConstantType:     jsii.String("constantType"),
								SingularConstant: jsii.String("singularConstant"),
							},
							Inverse: jsii.Bool(false),
						},
						DateRangeFilter: &awsquicksight.CfnTopic_TopicDateRangeFilterProperty{
							Constant: &awsquicksight.CfnTopic_TopicRangeFilterConstantProperty{
								ConstantType: jsii.String("constantType"),
								RangeConstant: &awsquicksight.CfnTopic_RangeConstantProperty{
									Maximum: jsii.String("maximum"),
									Minimum: jsii.String("minimum"),
								},
							},
							Inclusive: jsii.Bool(false),
						},
						FilterClass:       jsii.String("filterClass"),
						FilterDescription: jsii.String("filterDescription"),
						FilterSynonyms: &[]*string{
							jsii.String("filterSynonyms"),
						},
						FilterType: jsii.String("filterType"),
						NumericEqualityFilter: &awsquicksight.CfnTopic_TopicNumericEqualityFilterProperty{
							Aggregation: jsii.String("aggregation"),
							Constant: &awsquicksight.CfnTopic_TopicSingularFilterConstantProperty{
								ConstantType:     jsii.String("constantType"),
								SingularConstant: jsii.String("singularConstant"),
							},
						},
						NumericRangeFilter: &awsquicksight.CfnTopic_TopicNumericRangeFilterProperty{
							Aggregation: jsii.String("aggregation"),
							Constant: &awsquicksight.CfnTopic_TopicRangeFilterConstantProperty{
								ConstantType: jsii.String("constantType"),
								RangeConstant: &awsquicksight.CfnTopic_RangeConstantProperty{
									Maximum: jsii.String("maximum"),
									Minimum: jsii.String("minimum"),
								},
							},
							Inclusive: jsii.Bool(false),
						},
						RelativeDateFilter: &awsquicksight.CfnTopic_TopicRelativeDateFilterProperty{
							Constant: &awsquicksight.CfnTopic_TopicSingularFilterConstantProperty{
								ConstantType:     jsii.String("constantType"),
								SingularConstant: jsii.String("singularConstant"),
							},
							RelativeDateFilterFunction: jsii.String("relativeDateFilterFunction"),
							TimeGranularity:            jsii.String("timeGranularity"),
						},
					},
				},
				NamedEntities: []interface{}{
					&awsquicksight.CfnTopic_TopicNamedEntityProperty{
						EntityName: jsii.String("entityName"),

						// the properties below are optional
						Definition: []interface{}{
							&awsquicksight.CfnTopic_NamedEntityDefinitionProperty{
								FieldName: jsii.String("fieldName"),
								Metric: &awsquicksight.CfnTopic_NamedEntityDefinitionMetricProperty{
									Aggregation: jsii.String("aggregation"),
									AggregationFunctionParameters: map[string]*string{
										"aggregationFunctionParametersKey": jsii.String("aggregationFunctionParameters"),
									},
								},
								PropertyName:  jsii.String("propertyName"),
								PropertyRole:  jsii.String("propertyRole"),
								PropertyUsage: jsii.String("propertyUsage"),
							},
						},
						EntityDescription: jsii.String("entityDescription"),
						EntitySynonyms: &[]*string{
							jsii.String("entitySynonyms"),
						},
						SemanticEntityType: &awsquicksight.CfnTopic_SemanticEntityTypeProperty{
							SubTypeName: jsii.String("subTypeName"),
							TypeName:    jsii.String("typeName"),
							TypeParameters: map[string]*string{
								"typeParametersKey": jsii.String("typeParameters"),
							},
						},
					},
				},
			},
		},
		Description: jsii.String("description"),
		FolderArns: &[]*string{
			jsii.String("folderArns"),
		},
		Name: jsii.String("name"),
		Tags: &[]*awscdk.CfnTag{
			{
				Key:   jsii.String("key"),
				Value: jsii.String("value"),
			},
		},
		TopicId:               jsii.String("topicId"),
		UserExperienceVersion: jsii.String("userExperienceVersion"),
	})

	fmt.Printf("Topic created: %v\n", cfnTopic)
	return stack
}

func main() {
	app := awscdk.NewApp(nil)

	NewQuickSightTopicStack(app, "QuickSightTopicStack", &QuickSightTopicStackProps{
		StackProps: awscdk.StackProps{
			Env: env(),
		},
	})

	app.Synth(nil)
}

func env() *awscdk.Environment {
	return &awscdk.Environment{
		Account: jsii.String("123456789012"),
		Region:  jsii.String("us-east-1"),
	}
}
