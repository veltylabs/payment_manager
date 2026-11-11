package paymentmanager

const (
	StatusStarted    = "started"
	ReconcilePending = "pending"
	ReconcileDone    = "done"
)

// QueryPayment y QueryStatus son los parámetros de la query con que la ruta de retorno de la pasarela devuelve al cliente a la app.
const (
	QueryPayment = "payment"
	QueryStatus  = "status"
)
