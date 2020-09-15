package exporter

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

// Exporter implements the prometheus.Exporter interface, and exports AWS AutoScaling metrics.
type Exporter struct {
	session         *session.Session
	groups          []string
	duration        prometheus.Gauge
	scrapeErrors    prometheus.Gauge
	totalScrapes    prometheus.Counter
	groupMetrics    map[string]*prometheus.GaugeVec
	instanceMetrics map[string]*prometheus.GaugeVec
	metricsMtx      sync.RWMutex
	sync.RWMutex
}

type GroupScrapeResult struct {
	Name             string
	Value            float64
	AutoScalingGroup string
	Region           string
}

// NewExporter returns a new exporter of AWS Autoscaling group metrics.
func NewExporter(region string, groups []string) (*Exporter, error) {

	session, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		log.WithError(err).Error("Error creating AWS session")
		return nil, err
	}

	e := Exporter{
		session:        session,
		groups:         groups,
		duration: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "aws_autoscaling",
			Name:      "scrape_duration_seconds",
			Help:      "The scrape duration.",
		}),
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "aws_autoscaling",
			Name:      "scrapes_total",
			Help:      "Total AWS autoscaling group scrapes.",
		}),
		scrapeErrors: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "aws_autoscaling",
			Name:      "scrape_error",
			Help:      "The scrape error status.",
		}),
	}

	e.initGauges()
	return &e, nil
}

func (e *Exporter) initGauges() {
	e.groupMetrics = map[string]*prometheus.GaugeVec{}

	e.groupMetrics["pending_instances_total"] = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "aws_autoscaling",
		Name:      "pending_instances_total",
		Help:      "Total number of pending instances in the auto scaling group",
	}, []string{"asg_name", "region"})
	e.groupMetrics["inservice_instances_total"] = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "aws_autoscaling",
		Name:      "inservice_instances_total",
		Help:      "Total number of in service instances in the auto scaling group",
	}, []string{"asg_name", "region"})
	e.groupMetrics["standby_instances_total"] = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "aws_autoscaling",
		Name:      "standby_instances_total",
		Help:      "Total number of standby instances in the auto scaling group",
	}, []string{"asg_name", "region"})
	e.groupMetrics["terminating_instances_total"] = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "aws_autoscaling",
		Name:      "terminating_instances_total",
		Help:      "Total number of terminating instances in the auto scaling group",
	}, []string{"asg_name", "region"})
	e.groupMetrics["spot_instances_total"] = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "aws_autoscaling",
		Name:      "spot_instances_total",
		Help:      "Total number of spot instances in the auto scaling group",
	}, []string{"asg_name", "region"})
	e.groupMetrics["instances_total"] = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "aws_autoscaling",
		Name:      "instances_total",
		Help:      "Total number of instances in the auto scaling group",
	}, []string{"asg_name", "region"})

}

// Describe outputs metric descriptions.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range e.groupMetrics {
		m.Describe(ch)
	}
	for _, m := range e.instanceMetrics {
		m.Describe(ch)
	}
	ch <- e.duration.Desc()
	ch <- e.totalScrapes.Desc()
	ch <- e.scrapeErrors.Desc()
}

// Collect fetches info from the AWS API and the BanzaiCloud recommendation API
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {

	groupScrapes := make(chan GroupScrapeResult)

	e.Lock()
	defer e.Unlock()

	e.initGauges()
	go e.scrape(groupScrapes)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		e.setGroupMetrics(groupScrapes)
	}()
	wg.Wait()

	e.duration.Collect(ch)
	e.totalScrapes.Collect(ch)
	e.scrapeErrors.Collect(ch)

	for _, m := range e.groupMetrics {
		m.Collect(ch)
	}
}

func (e *Exporter) scrape(groupScrapes chan<- GroupScrapeResult) {

	defer close(groupScrapes)
	now := time.Now().UnixNano()
	e.totalScrapes.Inc()

	var errorCount uint64 = 0

	asgSvc := autoscaling.New(e.session, aws.NewConfig())
	err := asgSvc.DescribeAutoScalingGroupsPages(&autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: aws.StringSlice(e.groups),
	}, func(result *autoscaling.DescribeAutoScalingGroupsOutput, lastPage bool) bool {
		log.Debugf("Number of AutoScaling Groups found: %d [lastPage = %t]", len(result.AutoScalingGroups), lastPage)
		var wg sync.WaitGroup
		for _, asg := range result.AutoScalingGroups {
			log.Debug("scraping: ", *asg.AutoScalingGroupName)
			wg.Add(1)
			go func(asg *autoscaling.Group) {
				defer wg.Done()
				describeLcOutput, err := asgSvc.DescribeLaunchConfigurations(&autoscaling.DescribeLaunchConfigurationsInput{
					LaunchConfigurationNames: []*string{asg.LaunchConfigurationName},
				})
				if err != nil {
					log.WithField("autoScalingGroup", *asg.AutoScalingGroupName).WithError(err).Error("Failed to fetch launch configuration for auto scaling group, recommendation related metrics will not be reported.")
					atomic.AddUint64(&errorCount, 1)
				} else if len(describeLcOutput.LaunchConfigurations) != 1 {
					log.WithField("autoScalingGroup", *asg.AutoScalingGroupName).Error("Failed to fetch launch configuration for auto scaling group, recommendation related metrics will not be reported.")
					atomic.AddUint64(&errorCount, 1)
				} else {
					if err != nil {
						log.WithField("autoScalingGroup", *asg.AutoScalingGroupName).WithError(err).Error("Failed to get recommendations, recommendation related metrics will not be reported.")
						atomic.AddUint64(&errorCount, 1)
					}
				}
				if err := e.scrapeAsg(groupScrapes, asg); err != nil {
					log.WithField("autoScalingGroup", *asg.AutoScalingGroupName).Error(err)
					atomic.AddUint64(&errorCount, 1)

				}
			}(asg)
		}
		wg.Wait()
		return true
	})
	if err != nil {
		log.WithError(err).Error("An error happened while fetching AutoScaling Groups")
		atomic.AddUint64(&errorCount, 1)
	}

	e.scrapeErrors.Set(float64(atomic.LoadUint64(&errorCount)))
	e.duration.Set(float64(time.Now().UnixNano()-now) / 1000000000)
}

func (e *Exporter) setGroupMetrics(scrapes <-chan GroupScrapeResult) {
	log.Debug("set group metrics")
	for scr := range scrapes {
		name := scr.Name
		if _, ok := e.groupMetrics[name]; !ok {
			e.metricsMtx.Lock()
			e.groupMetrics[name] = prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Namespace: "aws_autoscaling",
				Name:      name,
			}, []string{"asg_name", "region"})
			e.metricsMtx.Unlock()
		}
		var labels prometheus.Labels = map[string]string{"asg_name": scr.AutoScalingGroup, "region": scr.Region}
		e.groupMetrics[name].With(labels).Set(float64(scr.Value))
	}
}

func (e *Exporter) scrapeAsg(groupScrapes chan<- GroupScrapeResult, asg *autoscaling.Group) error {
	log.WithField("autoScalingGroup", *asg.AutoScalingGroupName).Debug("getting metrics from the auto scaling group")

	var pendingInstances, inServiceInstances, standbyInstances, terminatingInstances int
	var instanceIds []*string

	if len(asg.Instances) > 0 {
		for _, inst := range asg.Instances {
			switch *inst.LifecycleState {
			case "InService":
				inServiceInstances++
			case "Pending":
				pendingInstances++
			case "Terminating":
				terminatingInstances++
			case "Standby":
				standbyInstances++
			}
			instanceIds = append(instanceIds, inst.InstanceId)
		}
	}

	groupScrapes <- GroupScrapeResult{
		Name:             "instances_total",
		Value:            float64(len(asg.Instances)),
		AutoScalingGroup: *asg.AutoScalingGroupName,
		Region:           *e.session.Config.Region,
	}
	groupScrapes <- GroupScrapeResult{
		Name:             "pending_instances_total",
		Value:            float64(pendingInstances),
		AutoScalingGroup: *asg.AutoScalingGroupName,
		Region:           *e.session.Config.Region,
	}
	groupScrapes <- GroupScrapeResult{
		Name:             "inservice_instances_total",
		Value:            float64(inServiceInstances),
		AutoScalingGroup: *asg.AutoScalingGroupName,
		Region:           *e.session.Config.Region,
	}
	groupScrapes <- GroupScrapeResult{
		Name:             "terminating_instances_total",
		Value:            float64(terminatingInstances),
		AutoScalingGroup: *asg.AutoScalingGroupName,
		Region:           *e.session.Config.Region,
	}
	groupScrapes <- GroupScrapeResult{
		Name:             "standby_instances_total",
		Value:            float64(standbyInstances),
		AutoScalingGroup: *asg.AutoScalingGroupName,
		Region:           *e.session.Config.Region,
	}

	if len(instanceIds) > 0 {
		var err error
		log.WithField("autoScalingGroup", *asg.AutoScalingGroupName).Debug("getting metrics from the instances in the autoscaling group")
		if err != nil {
			return err
		}
	}
	return nil
}
