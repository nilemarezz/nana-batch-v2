package model

type DataSheet struct {
	Order_Timestamp string
	Account         string
	Address         string
	Product         string
	Shipping        string
	SkipFlag        string
	Shipping_fee    float64
	Express_fee     float64
	Payment_fee     float64
	Notice          string
	TrackingNo      string
	SendDate        string
}

type PledgeSheet struct {
	Order_Timestamp string
	Account         string
	Product         string
	Total           float64
	Notice          string
}
