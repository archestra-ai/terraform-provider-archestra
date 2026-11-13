# Import an existing cost export by UUID. start_from is not returned by the API;
# setting it after import records the value without replacing the export.
terraform import archestra_cost_export.daily 11111111-1111-4111-8111-111111111111
