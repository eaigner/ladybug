### ladybug


Ladybug is a simple Heroku hosted tool for customers to report issues. Attachments are uploaded to S3 and an issue is created on GitHub for the specified repository (can also be private). It was originally written to help with support for [PSPDFKit](http://pspdfkit.com).

To use it you have to set a few environment variables on Heroku. Those are:

* `PORT` (optional): Set this for a local environment, Heroku will set this for you in production
* `GH_TOKEN`: Your GitHub auth token
* `GH_REPO`: Your GitHub repository (e.g. `eaigner/ladybug`)
* `S3_BUCKET`: The S3 bucket name to upload attachments to
* `S3_KEY`: The S3 access key
* `S3_SECRET`: The S3 secret
* `S3_PATH`: The folder to place attachments in or `/`


