## AWS Autoscaling exporter

Prometheus exporter for AWS auto scaling groups, forked from https://github.com/banzaicloud/aws-autoscaling-exporter project. In contrast to the parent project this fork has been reduced to not access their spot-recommender anymore.

Provides auto scaling group level metrics similar to CloudWatch metrics. For group level metrics the exporter is polling the AWS APIs for auto scaling groups.

### Quick start

Building the project is as simple as running a go build command. The result is a statically linked executable binary.
```
go build .
```

The following options can be configured when starting the exporter:

```
./aws-autoscaling-exporter --help
Usage of ./aws-autoscaling-exporter:
  -auto-scaling-groups
        Comma separated list of auto scaling groups to monitor. Empty value means all groups in the region.
  -groups-filter
        Regex filter to limit the ASGs that are scraped. If `-groups-filter` is set `-auto-scaling-groups` is being ignored.
        Examples: '.*' or 'SomeName-[^/]-SomeOtherString' 
  -listen-address string
        The address to listen on for HTTP requests. (default ":8089")
  -log-level string
        log level (default "info")
  -metrics-path string
        path to metrics endpoint (default "/metrics")
  -region string
        AWS region that the exporter should query (default "eu-west-1")
```

### Metrics

```
# HELP aws_autoscaling_inservice_instances_total Total number of in service instances in the auto scaling group
# TYPE aws_autoscaling_inservice_instances_total gauge
aws_autoscaling_inservice_instances_total{asg_name="marci-test",region="eu-west-1"} 0
# HELP aws_autoscaling_instances_total Total number of instances in the auto scaling group
# TYPE aws_autoscaling_instances_total gauge
aws_autoscaling_instances_total{asg_name="marci-test",region="eu-west-1"} 1
# HELP aws_autoscaling_pending_instances_total Total number of pending instances in the auto scaling group
# TYPE aws_autoscaling_pending_instances_total gauge
aws_autoscaling_pending_instances_total{asg_name="marci-test",region="eu-west-1"} 1
# HELP aws_autoscaling_scrape_duration_seconds The scrape duration.
# TYPE aws_autoscaling_scrape_duration_seconds gauge
aws_autoscaling_scrape_duration_seconds 0.592821
# HELP aws_autoscaling_scrape_error The scrape error status.
# TYPE aws_autoscaling_scrape_error gauge
aws_autoscaling_scrape_error 0
# HELP aws_autoscaling_scrapes_total Total AWS autoscaling group scrapes.
# TYPE aws_autoscaling_scrapes_total counter
aws_autoscaling_scrapes_total 15
# HELP aws_autoscaling_spot_instances_total Total number of spot instances in the auto scaling group
# TYPE aws_autoscaling_spot_instances_total gauge
aws_autoscaling_spot_instances_total{asg_name="marci-test",region="eu-west-1"} 1
# HELP aws_autoscaling_standby_instances_total Total number of standby instances in the auto scaling group
# TYPE aws_autoscaling_standby_instances_total gauge
aws_autoscaling_standby_instances_total{asg_name="marci-test",region="eu-west-1"} 0
# HELP aws_autoscaling_terminating_instances_total Total number of terminating instances in the auto scaling group
# TYPE aws_autoscaling_terminating_instances_total gauge
aws_autoscaling_terminating_instances_total{asg_name="marci-test",region="eu-west-1"} 0
```
