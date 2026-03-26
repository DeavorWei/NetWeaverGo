package executor

func reduceEffects(reducer *SessionReducer, event SessionEvent) []SessionEffect {
	if reducer == nil {
		return nil
	}
	batch := reducer.ReduceBatch(event)
	if batch == nil {
		return nil
	}
	return batch.Effects
}

func feedEffects(adapter *SessionAdapter, chunk string) []SessionEffect {
	if adapter == nil {
		return nil
	}
	batch := adapter.FeedTransitionBatch(chunk)
	if batch == nil {
		return nil
	}
	return batch.Effects
}
