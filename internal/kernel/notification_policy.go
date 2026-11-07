// pattern: Functional Core
package kernel

func notificationTestReceipt(adapter, intentID, deliveryID, state string) NotificationTestReceipt {
	return NotificationTestReceipt{IntentID: intentID, DeliveryID: deliveryID, Adapter: adapter, State: state}
}
