package main

import (
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"fmt"
	"time"
	"flag"
	"sort"
)

type Instances []*ec2.Instance

var flag_asgName string
var flag_deleteInstances bool
var flag_region string
var flag_list bool
var flag_numInstances int
var flag_percentInsts float64

var now time.Time

func init() {
	flag.StringVar(&flag_asgName, "n", "", "name of autoscaling group. if no name provided then all autoscaling grouops are considered")
//	flag.StringVar(&flag_asgName, "name", "", "name of autoscaling group")
	flag.StringVar(&flag_region, "r", "us-east-1", "aws region")
//	flag.StringVar(&flag_region, "region", "us-east-1", "aws region")
	flag.IntVar(&flag_numInstances, "i", 0, "number of instances to print (has priority over percentage)")
//	flag.IntVar(&flag_numInstances, "instances", 0, "number of instances to print (has priority over percentage)")
	flag.Float64Var(&flag_percentInsts, "p", 0.0, "percentage of instances to print (minimum of one instance is printed)")
//	flag.Float64Var(&flag_percentInsts, "percentage", 0.0, "percentage of instances to print (minimum of one instance is printed)")
}

// Returns all instances that are in autoscaling groups within the given region
// if a autoscaling group name is specified then only the instances within that group are returned
func getAutoScalingInstances() []*autoscaling.InstanceDetails{
	instances := make([]*autoscaling.InstanceDetails, 0, 50)
	var token *string
	var resp *autoscaling.DescribeAutoScalingInstancesOutput
	var err error
	
	svc := autoscaling.New(session.New(), &aws.Config{ Region: aws.String(flag_region)} )

	// DescribeAutoscalinginstances() only returns 50 instances a time so...
	// keep calling for more until all have been retrieved
	for {
	
		params := &autoscaling.DescribeAutoScalingInstancesInput{
			NextToken: token,
		}
		resp, err = svc.DescribeAutoScalingInstances(params)
		if err != nil {
			fmt.Println(err.Error())
			return nil
		}
		instances = append(instances, resp.AutoScalingInstances...)
		token = resp.NextToken
		
		// no more instances to get. We done hehr
		if token == nil {
			break
		}
	}
	if flag_asgName != "" {
		return filterByASGName(instances)
	}
	return instances
}

// Return a list of instanceDetails that are part of asg named flag_asgName.
func filterByASGName(instances []*autoscaling.InstanceDetails) []*autoscaling.InstanceDetails {
	newList := make([]*autoscaling.InstanceDetails, 0)
	for _, instance := range instances {
		if *instance.AutoScalingGroupName == flag_asgName {
			newList = append(newList, instance)
		}
	}
	return newList
}

// Extracts a list of the instance ids from the autoscaling instance details.
func getInstanceIds(instances []*autoscaling.InstanceDetails) []*string {
	ids := make([]*string, 0)
	for _, instance := range instances {
		ids = append(ids, instance.InstanceId)
	}

	return ids
}

// Returns a list of ec2 instances given a list of instance ids
// this is needed because the autoscaling group instances provide as many details
// as the ec2 instances do. In this case we need the instance launch time.
func getEC2Instances(ids []*string) []*ec2.Instance {
	instances := make([]*ec2.Instance, 0)
	svc := ec2.New(session.New(), &aws.Config{ Region: aws.String(flag_region) })
	params := &ec2.DescribeInstancesInput{ InstanceIds: ids }

	resp, err := svc.DescribeInstances(params)
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}
	for _, reservation := range resp.Reservations {
		for _, i := range reservation.Instances {
			instances = append(instances, i)
		}
	}
	return instances
}

func printNum(instances Instances, max int) {
	if max > len(instances) {
		max = len(instances)
	}
	for i := 0; i < max; i++ {
		fmt.Println(*instances[i].InstanceId)
	}
}

func printPercent(instances Instances) {
	max := int(flag_percentInsts * float64(len(instances)))
	// lets just always print one as a minimum
	if max == 0 {
		max = 1
	}
	printNum(instances, max)
}

func (instances Instances) Len() int {return len(instances)}
func (instances Instances) Swap(i, j int) {instances[i], instances[j] = instances[j], instances[i]}
func (instances Instances) Less(i, j int) bool {
	iDur := now.Sub(*instances[i].LaunchTime)
	jDur := now.Sub(*instances[j].LaunchTime)
	return iDur < jDur
}

func main() {
	flag.Parse()
	now = time.Now() // global variable used for sorting the instances by launch time

	insts := Instances(getEC2Instances(getInstanceIds(getAutoScalingInstances())))
	sort.Sort(insts)    // sort by launch date
	sort.Reverse(insts) // reverse order so that olderst instances are first
	if flag_numInstances != 0 {
		printNum(insts, flag_numInstances)
	} else if flag_percentInsts != 0.0 {
		printPercent(insts)
	} else {
		printNum(insts, len(insts))
	}
}
