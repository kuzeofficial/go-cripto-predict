package main

import (
	"fmt"
	"math"
	"time"

	"github.com/sajari/regression"
	gecko "github.com/superoo7/go-gecko/v3"
	"github.com/superoo7/go-gecko/v3/types"
)
const (
	COIN = "chainlink"
)

type CryptoData struct {
	Time       string  `json:"time"`
	Price      float64 `json:"price"`
	Volume     float64 `json:"volume"`
	MarketCap  float64 `json:"market_cap"`
}
type RegressionModel struct {
	Model *regression.Regression
}

type PreprocessedData struct {
	Time      string  `json:"time"`
	Price     float64 `json:"price"`
	Volume    float64 `json:"volume"`
	MarketCap float64 `json:"market_cap"`
}

func main() {
	// Load your historical and supplemental data for model training
	trainingCryptoData, testingCryptoData := loadHistoricalData()

	// Preprocess the training and testing data
	preprocessedTrainingData, preprocessedTestingData := preprocessData(trainingCryptoData, testingCryptoData)

	// Choose and train the machine learning model(s)
	model := trainModel(preprocessedTrainingData)

	// Evaluate the performance of trained models
	mse := evaluateModel(model, preprocessedTestingData)
	fmt.Printf("Model evaluation results (MSE): %v\n", mse)

	// Initiate the real-time price prediction analysis loop
	realTimePredictionsLoop(model, time.Second * 5)
}

func convertToCryptoData(prices, marketCaps, totalVolumes []types.ChartItem) []CryptoData {
	cryptoData := make([]CryptoData, len(prices))

	for i := range prices {
		cryptoData[i] = CryptoData{
			Time:      time.Unix(int64(prices[i][0]/1000), 0).Format(time.RFC3339),
			Price:     float64(prices[i][1]),
			Volume:    float64(totalVolumes[i][1]),
			MarketCap: float64(marketCaps[i][1]),
		}
	}
	
	return cryptoData
}

func loadHistoricalData() (trainingCryptoData []CryptoData, testingCryptoData []CryptoData) {
    // coin := "gas" // Example: use bitcoin
    currency := "usd" // Example: use USD for price info
    // days := 365       // Example: fetch data from last can365 days
    cg := gecko.NewClient(nil)

    // Fetch historical price, volume, and market cap data in USD
    historicalData, err := cg.CoinsIDMarketChart(COIN, currency, "max")
    if err != nil {
        fmt.Println("Error retrieving historical data:", err)
        return
    }

    // Divide dataset into training (80%) and testing (20%) sets
    prices := *historicalData.Prices
    marketCaps := *historicalData.MarketCaps
    totalVolumes := *historicalData.TotalVolumes

    splitIndex := int(float64(len(prices)) * 0.8)

    trainingPrices := prices[:splitIndex]
    testingPrices := prices[splitIndex:]

    trainingMarketCaps := marketCaps[:splitIndex]
    testingMarketCaps := marketCaps[splitIndex:]

    trainingTotalVolumes := totalVolumes[:splitIndex]
    testingTotalVolumes := totalVolumes[splitIndex:]

    trainingCryptoData = convertToCryptoData(trainingPrices, trainingMarketCaps, trainingTotalVolumes)
    testingCryptoData = convertToCryptoData(testingPrices, testingMarketCaps, testingTotalVolumes)
    return
}

func preprocessData(trainingCryptoData []CryptoData, testingCryptoData []CryptoData) (preprocessedTrainingData []PreprocessedData, preprocessedTestingData []PreprocessedData) {
	combinedData := append(trainingCryptoData, testingCryptoData...)

	minPrice, maxPrice := findMinMax(combinedData, "Price")
	minVolume, maxVolume := findMinMax(combinedData, "Volume")
	minMarketCap, maxMarketCap := findMinMax(combinedData, "MarketCap")
	
	preprocessedTrainingData = make([]PreprocessedData, len(trainingCryptoData))
	preprocessedTestingData = make([]PreprocessedData, len(testingCryptoData))

	for i, data := range trainingCryptoData {
		preprocessedTrainingData[i] = PreprocessedData{
			Time:      data.Time,
			Price:     normalize(data.Price, minPrice, maxPrice),
			Volume:    normalize(data.Volume, minVolume, maxVolume),
			MarketCap: normalize(data.MarketCap, minMarketCap, maxMarketCap),
		}
	}

	for i, data := range testingCryptoData {
		preprocessedTestingData[i] = PreprocessedData{
			Time:      data.Time,
			Price:     normalize(data.Price, minPrice, maxPrice),
			Volume:    normalize(data.Volume, minVolume, maxVolume),
			MarketCap: normalize(data.MarketCap, minMarketCap, maxMarketCap),
		}
	}

	return
}

func findMinMax(data []CryptoData, attribute string) (float64, float64) {
	min := math.MaxFloat64
	max := -math.MaxFloat64

	for _, item := range data {
		var value float64
		switch attribute {
		case "Price":
			value = item.Price
		case "Volume":
			value = item.Volume
		case "MarketCap":
			value = item.MarketCap
		default:
			continue
		}

		if value < min {
			min = value
		}
		if value > max {
			max = value
		}
	}

	return min, max
}

func normalize(value, minValue, maxValue float64) float64 {
	return (value - minValue) / (maxValue - minValue)
}

func trainModel(preprocessedTrainingData []PreprocessedData) *RegressionModel {
	regModel := new(regression.Regression)
	regModel.SetObserved("Price")
	regModel.SetVar(0, "MarketCap")

	for _, data := range preprocessedTrainingData {
		Y := data.Price
		X := []float64{data.MarketCap}
		regModel.Train(regression.DataPoint(Y, X))
	}

	err := regModel.Run()
	if err != nil {
		fmt.Println("Error training the model:", err)
		return nil
	}

	fmt.Printf("Trained Model Summary: \n%s\n", regModel)

	return &RegressionModel{
		Model: regModel,
	}
}

func evaluateModel(regModel *RegressionModel, preprocessedTestingData []PreprocessedData) float64 {
	mse := 0.0
	n := len(preprocessedTestingData)

	for _, data := range preprocessedTestingData {
		observed := data.Price
		predicted, _ := regModel.Model.Predict([]float64{data.MarketCap})
		mse += math.Pow(observed-predicted, 2)
	}

	// Calculate the mean squared error
	mse /= float64(n)

	// Return the mean squared error as the evaluation metric
	return mse
}

func realTimePredictionsLoop(regModel *RegressionModel, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	cg := gecko.NewClient(nil)
	currency := "usd"

	for {
		select {
		case <-ticker.C:
			latestCryptoData, err := fetchLatestData(cg, COIN, currency)
			if err != nil {
				fmt.Println("Error fetching latest data:", err)
				continue
			}
			pricePrediction, _ := regModel.Model.Predict([]float64{latestCryptoData.Price})
			fmt.Printf("Real-time Predicted Price: %.2f, Actual Price: %.2f\n", pricePrediction, latestCryptoData.Price)
		}
	}
}

func fetchLatestData(cg *gecko.Client, coin, currency string) (latestCryptoData CryptoData, err error) {
    historicalData, err := cg.CoinsIDMarketChart(coin, currency, "1")
	if err != nil {
		return
	}

	prices := *historicalData.Prices
	marketCaps := *historicalData.MarketCaps
	totalVolumes := *historicalData.TotalVolumes
	latestPrice := prices[len(prices) - 1]
	latestMarketCap :=  marketCaps[len(marketCaps) -1]
	latestTotalVolume :=  totalVolumes[len(totalVolumes) -1]


	latestCryptoData = CryptoData{
		Time:      time.Unix(int64(latestPrice[0]/1000), 0).Format(time.RFC3339),
		Price:     float64(latestPrice[1]),
		Volume:    float64(latestTotalVolume[1]),
		MarketCap: float64(latestMarketCap[1]),
	}
	return 
}