## Template

### To be named

Templates have two parts: evaluation rules and permissions rules.

The update of each part will result in a new revision of template.

When calculating the scores of a scorecard, we always use the as-of evaluation rules, which are the rules from when a scorecard is created.

When applying permissions, we always use the latest revision's permission rules, plus special adjustments. For example, when we first enable editing in revision A and then disable it in revision B, we need to consider whether scorecards created by revision A should be updated with new permission settings. We also need to consider each permission individually. It's a heavy workload.


### Missing schema version number and updater

We keep adding new features to template. Therefore the `template` column in db, which is JSON, keeps changing its structure.

Currently, what we're doing is adding code to handle the migration. But the migration is scattered and can become outdated after a long time once most templates are using new versions.

A better approach could be versioning the JSON structure and having a migrator.

For example, if we have versions from 1 to 5, there should also be updaters that handle version 1 -> 2, 2 -> 3, 3 -> 4, and 4 -> 5. The FE should read the template from db and apply updaters sequentially to migrate. At the same time, the rest of FE can handle only the latest template version because only the latest version will be displayed.
