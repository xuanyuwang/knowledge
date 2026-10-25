package shared

import "github.com/cresta/shared-go/util/envflag"

// ExportChartLambdaFunctionARN is the shared Lambda function ARN used for exporting chart images.
// Used by both dashboarding (ExportAnalyticsChartImage) and realtimetrends (ExportTrendChart).
var ExportChartLambdaFunctionARN = envflag.String("EXPORT_ANALYTICS_CHART_IMAGE_LAMBDA_FUNCTION_ARN_RESOURCE", "arn:aws:lambda:us-west-2:515154039920:function:highcharts-export-lambda", "The full ARN resource of the Lambda function to use for exporting chart images")
