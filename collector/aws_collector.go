package collector

import (
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

const (
	namespace = "aws_spot_market_exporter"
)

type Exporter struct {
	session          *session.Session
	spotMarketPrices *prometheus.GaugeVec
	duration         prometheus.Gauge
	scrapeErrors     prometheus.Gauge
	totalScrapes     prometheus.Counter
	sync.RWMutex
}

type SpotMarketScrapeResult struct {
	Region           string
	AvailabilityZone string
	InstanceType     string
	Product          string
	Price            float64
}

type spotMarketScrapeError struct {
	count uint64
}

func (e *spotMarketScrapeError) Error() string {
	return fmt.Sprintf("Error count: %d", e.count)
}

// Describe add metrics to prometheus
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.duration.Desc()
	ch <- e.totalScrapes.Desc()
	ch <- e.scrapeErrors.Desc()
	e.spotMarketPrices.Describe(ch)
}

// NewExporter returns a new exporter of SpotMarket prices metrics.
func NewExporter() (*Exporter, error) {

	session, err := session.NewSession(&aws.Config{
		Region: aws.String("eu-west-1"),
	})
	if err != nil {
		log.Fatalf("failed to create session %v\n", err)
		return nil, err
	}

	e := Exporter{
		session: session,
		duration: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "scrape_duration_seconds",
			Help:      "The scrape duration.",
		}),
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "scrapes_total",
			Help:      "Total AWS autoscaling group scrapes.",
		}),
		scrapeErrors: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "scrape_error",
			Help:      "The scrape error status.",
		}),
	}

	e.initGauges()
	return &e, nil
}

func (e *Exporter) initGauges() {
	e.spotMarketPrices = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "spot_price",
		Help:      "Current market price of a spot instance, per hour,  in dollars",
	}, []string{"region", "az", "product", "instance_type"})
}

// Collect fetches info from the AWS API
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {

	spotScrapes := make(chan SpotMarketScrapeResult)

	e.Lock()
	defer e.Unlock()

	e.initGauges()
	go e.scrape(spotScrapes)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		e.setSpotMarketMetrics(spotScrapes)
	}()
	wg.Wait()

	e.duration.Collect(ch)
	e.totalScrapes.Collect(ch)
	e.scrapeErrors.Collect(ch)
	e.spotMarketPrices.Collect(ch)
}

func (e *Exporter) scrape(spotScrapes chan<- SpotMarketScrapeResult) {

	defer close(spotScrapes)
	now := time.Now().UnixNano()
	e.totalScrapes.Inc()

	var errorCount uint64

	ec2Svc := ec2.New(e.session, aws.NewConfig())

	regions, err := ec2Svc.DescribeRegions(&ec2.DescribeRegionsInput{})
	if err != nil {
		log.Errorln("There was an error listing all regions", err.Error())
		atomic.AddUint64(&errorCount, 1)
	} else {
		var wg sync.WaitGroup

		for _, region := range regions.Regions {
			wg.Add(1)
			go func(region string) {
				defer wg.Done()

				if err := e.scrapeSpotMarketPrice(spotScrapes, region); err != nil {
					log.Errorln("An error happened while fetching SpotMarket Prices", err.Error())
					if e, ok := err.(*spotMarketScrapeError); ok {
						atomic.AddUint64(&errorCount, e.count)
					} else {
						atomic.AddUint64(&errorCount, 1)
					}
				}
			}(*region.RegionName)
		}
		wg.Wait()
	}

	e.scrapeErrors.Set(float64(atomic.LoadUint64(&errorCount)))
	e.duration.Set(float64(time.Now().UnixNano()-now) / 1000000000)
}

func (e *Exporter) setSpotMarketMetrics(scrapes <-chan SpotMarketScrapeResult) {
	log.Debug("set spot market metrics")
	for scr := range scrapes {
		var labels prometheus.Labels = map[string]string{
			"region":        scr.Region,
			"az":            scr.AvailabilityZone,
			"product":       scr.Product,
			"instance_type": scr.InstanceType,
		}
		log.Debugf("Setting %v to %g", labels, scr.Price)
		e.spotMarketPrices.With(labels).Set(float64(scr.Price))
	}
}

func (e *Exporter) scrapeSpotMarketPrice(scrapes chan<- SpotMarketScrapeResult, region string) error {
	var errorCount uint64

	ec2Svc := ec2.New(e.session, &aws.Config{
		Region: aws.String(region),
	})

	phResp, err := ec2Svc.DescribeSpotPriceHistory(&ec2.DescribeSpotPriceHistoryInput{
		StartTime: aws.Time(time.Now()),
		EndTime:   aws.Time(time.Now()),
	})
	if err != nil {
		log.Errorln("There was an error querying AWS spot market", err.Error())
		errorCount++
	}

	for _, sp := range phResp.SpotPriceHistory {

		if sp.SpotPrice != nil {
			spotPrice, err := strconv.ParseFloat(*sp.SpotPrice, 64)
			if err != nil {
				log.Errorln("there was an error listing spot requests", err.Error())
				errorCount++
			} else {
				scrapes <- SpotMarketScrapeResult{
					Region:           region,
					AvailabilityZone: *sp.AvailabilityZone,
					Product:          *sp.ProductDescription,
					InstanceType:     *sp.InstanceType,
					Price:            spotPrice,
				}
			}
		}
	}

	if errorCount > 0 {
		return &spotMarketScrapeError{errorCount}
	}
	return nil
}
