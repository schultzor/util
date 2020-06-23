// find all the AWS Lambda function log events for a given function in the last N hours and print them to stdout
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	cwlogs "github.com/aws/aws-sdk-go/service/cloudwatchlogs"
)

const lambdaPrefix = "/aws/lambda/"

var client *cwlogs.CloudWatchLogs

func handle(e error) {
	if e != nil {
		fmt.Fprintf(os.Stderr, "error: %v", e)
		os.Exit(1)
	}
}

func listGroups() {
	err := client.DescribeLogGroupsPages(&cwlogs.DescribeLogGroupsInput{
		LogGroupNamePrefix: aws.String(lambdaPrefix),
	}, func(page *cwlogs.DescribeLogGroupsOutput, lastPage bool) bool {
		for _, x := range page.LogGroups {
			fmt.Println(strings.TrimPrefix(*x.LogGroupName, lambdaPrefix))
		}
		return !lastPage
	})
	handle(err)
}

func ParseMillis(epochMillis int64) time.Time {
	return time.Unix(epochMillis/1000, (epochMillis%1000)*1000)
}

func main() {
	var hours = flag.Int("hours", 24, "number of hours to look back in lambda logs")
	var region = flag.String("region", "us-west-2", "AWS region")
	flag.Parse()
	funcName := flag.Arg(0)

	client = cwlogs.New(session.Must(session.NewSession(&aws.Config{Region: region})))
	if funcName == "" {
		fmt.Fprintf(os.Stderr, "No function argument given, listing functions in %s:", *region)
		listGroups()
		os.Exit(0)
	}
	cutoff := time.Now().Add(-time.Hour * time.Duration(*hours))
	groupName := lambdaPrefix + funcName
	fmt.Fprintf(os.Stderr, "Finding streams for %s with last event time after %s\n", groupName, cutoff.Format(time.RFC3339))
	streams := []string{}
	err := client.DescribeLogStreamsPages(&cwlogs.DescribeLogStreamsInput{
		Descending:   aws.Bool(false), // print oldest log streams first
		LogGroupName: &groupName,
	}, func(page *cwlogs.DescribeLogStreamsOutput, lastPage bool) bool {
		for _, x := range page.LogStreams {
			if x.LastEventTimestamp != nil && ParseMillis(*x.LastEventTimestamp).After(cutoff) {
				if x.LogStreamName != nil {
					streams = append(streams, *x.LogStreamName)
				}
			}
		}
		return !lastPage
	})
	handle(err)
	for _, streamName := range streams {
		fmt.Fprintf(os.Stderr, "Getting log events in %s/%s\n", groupName, streamName)
		err := client.GetLogEventsPages(&cwlogs.GetLogEventsInput{
			LogGroupName:  &groupName,
			LogStreamName: &streamName,
		},
			func(page *cwlogs.GetLogEventsOutput, lastPage bool) bool {
				for _, e := range page.Events {
					fmt.Printf("%v %s", *e.Timestamp, *e.Message)
				}
				return !lastPage
			})
		handle(err)
	}

}
