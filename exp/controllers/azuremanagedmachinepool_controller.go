func TestNewAzureMachinePoolReconciler(t *testing.T) {
    // Create a new instance of the client and recorder
    fakeClient := fake.NewClientBuilder().Build()
    fakeRecorder := record.NewFakeRecorder(1)

    // Create a new instance of the reconciler
    reconciler := NewAzureMachinePoolReconciler(fakeClient, fakeRecorder, time.Minute, "my-filter-value")

    // Check that the reconciler was initialized correctly
    if reconciler.Client != fakeClient {
        t.Errorf("Expected client to be %v, but got %v", fakeClient, reconciler.Client)
    }
    if reconciler.Recorder != fakeRecorder {
        t.Errorf("Expected recorder to be %v, but got %v", fakeRecorder, reconciler.Recorder)
    }
    if reconciler.ReconcileTimeout != time.Minute {
        t.Errorf("Expected reconcile timeout to be %v, but got %v", time.Minute, reconciler.ReconcileTimeout)
    }
    if reconciler.WatchFilterValue != "my-filter-value" {
        t.Errorf("Expected watch filter value to be %v, but got %v", "my-filter-value", reconciler.WatchFilterValue)
    }
}
