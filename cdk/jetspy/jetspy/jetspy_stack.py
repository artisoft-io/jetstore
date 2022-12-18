from aws_cdk import (
    aws_ec2 as ec2, 
    aws_ecs as ecs,
    aws_rds as rds,
    aws_ecs_patterns as ecs_patterns,
    aws_stepfunctions as _aws_stepfunctions,
    aws_stepfunctions_tasks as _aws_stepfunctions_tasks,
    aws_lambda as _lambda,
    App, Duration, Stack
)
from constructs import Construct
class JetspyStack(Stack):

    def __init__(self, scope: Construct, construct_id: str, **kwargs) -> None:
        super().__init__(scope, construct_id, **kwargs)

       # Lambda Handlers Definitions

        submit_lambda = _lambda.Function(self, 'submitLambda',
                                         handler='lambda_function.lambda_handler',
                                         runtime=_lambda.Runtime.PYTHON_3_9,
                                         code=_lambda.Code.from_asset('lambdas/submit'))

        status_lambda = _lambda.Function(self, 'statusLambda',
                                         handler='lambda_function.lambda_handler',
                                         runtime=_lambda.Runtime.PYTHON_3_9,
                                         code=_lambda.Code.from_asset('lambdas/status'))

        # Step functions Definition

        submit_job = _aws_stepfunctions_tasks.LambdaInvoke(
            self, "Submit Job",
            lambda_function=submit_lambda,
            output_path="$.Payload",
        )

        wait_job = _aws_stepfunctions.Wait(
            self, "Wait 30 Seconds",
            time=_aws_stepfunctions.WaitTime.duration(
                Duration.seconds(30))
        )

        status_job = _aws_stepfunctions_tasks.LambdaInvoke(
            self, "Get Status",
            lambda_function=status_lambda,
            output_path="$.Payload",
        )

        fail_job = _aws_stepfunctions.Fail(
            self, "Fail",
            cause='AWS Batch Job Failed',
            error='DescribeJob returned FAILED'
        )

        succeed_job = _aws_stepfunctions.Succeed(
            self, "Succeeded",
            comment='AWS Batch Job succeeded'
        )

        # Create Chain

        definition = submit_job.next(wait_job)\
            .next(status_job)\
            .next(_aws_stepfunctions.Choice(self, 'Job Complete?')
                  .when(_aws_stepfunctions.Condition.string_equals('$.status', 'FAILED'), fail_job)
                  .when(_aws_stepfunctions.Condition.string_equals('$.status', 'SUCCEEDED'), succeed_job)
                  .otherwise(wait_job))

        # Create state machine
        sm = _aws_stepfunctions.StateMachine(
            self, "StateMachine",
            definition=definition,
            timeout=Duration.minutes(5),
        )

        # # Create the Fargate cluster and service
        # vpc = ec2.Vpc(self, "jetspy", max_azs=3)     # default is all AZs in region

        # ecsCluster = ecs.Cluster(self, "jetspyCluster", vpc=vpc)

        # ecs_patterns.ApplicationLoadBalancedFargateService(self, "jetspyService",
        #     cluster=ecsCluster,            # Required
        #     cpu=512,                    # Default is 256
        #     desired_count=1,            # Default is 1
        #     task_image_options=ecs_patterns.ApplicationLoadBalancedTaskImageOptions(
        #         image=ecs.ContainerImage.from_registry("amazon/amazon-ecs-sample")),
        #     memory_limit_mib=2048,      # Default is 512
        #     public_load_balancer=True)  # Default is True

        # Create db cluster
        vpc = ec2.Vpc(self, "jetspy", max_azs=3)     # default is all AZs in region
        cluster = rds.ServerlessCluster(self, "AuroraCluster",
            engine=rds.DatabaseClusterEngine.aurora_postgres(version=rds.AuroraPostgresEngineVersion.VER_14_5),
            vpc=vpc,
            # credentials={"username": "clusteradmin"},
            cluster_identifier="db-endpoint-test",
            default_database_name="demos"
        )