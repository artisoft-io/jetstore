package main

import (
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
)

func DescribeTasks(svc *ecs.ECS, clusterName *string, tasksArn []*string) (*ecs.DescribeTasksOutput, error) {
	input := &ecs.DescribeTasksInput{
		Cluster: clusterName,
		Tasks: tasksArn,
	}

	result, err := svc.DescribeTasks(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ecs.ErrCodeServerException:
				fmt.Println(ecs.ErrCodeServerException, aerr.Error())
			case ecs.ErrCodeClientException:
				fmt.Println(ecs.ErrCodeClientException, aerr.Error())
			case ecs.ErrCodeInvalidParameterException:
				fmt.Println(ecs.ErrCodeInvalidParameterException, aerr.Error())
			case ecs.ErrCodeClusterNotFoundException:
				fmt.Println(ecs.ErrCodeClusterNotFoundException, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
		return nil, err
	}

	// fmt.Println(result)
	return result, nil
}

func main() {
	clusterName := aws.String("arn:aws:ecs:us-east-1:470601442608:cluster/JetstoreOneStack-ecsCluster15812518-H6LHXbi9i1dC")
	sess, err := session.NewSession()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	svc := ecs.New(sess)
	input := &ecs.ListTasksInput{
		Cluster: clusterName,
	}

	taskList, err := svc.ListTasks(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ecs.ErrCodeServerException:
				fmt.Println(ecs.ErrCodeServerException, aerr.Error())
			case ecs.ErrCodeClientException:
				fmt.Println(ecs.ErrCodeClientException, aerr.Error())
			case ecs.ErrCodeInvalidParameterException:
				fmt.Println(ecs.ErrCodeInvalidParameterException, aerr.Error())
			case ecs.ErrCodeClusterNotFoundException:
				fmt.Println(ecs.ErrCodeClusterNotFoundException, aerr.Error())
			case ecs.ErrCodeServiceNotFoundException:
				fmt.Println(ecs.ErrCodeServiceNotFoundException, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
		return
	}

	// fmt.Println(taskList)

	result, err := DescribeTasks(svc, clusterName, taskList.TaskArns)
	if err != nil {
		log.Panicln("while calling DescribeTasks", err.Error())
	}
	fmt.Println(result)

	for _, task := range result.Tasks {
		for _, container := range task.Containers {
			fmt.Printf("Container: Name: %s, LastStatus: %s", *container.Name, *container.LastStatus)
			if container.ExitCode != nil {
				fmt.Printf(", ExistCode: %d", *container.ExitCode)
			}
			if container.Reason != nil {
				fmt.Printf(", Reason: %d", *container.Reason)
			}
			fmt.Println()
		}
	}
}