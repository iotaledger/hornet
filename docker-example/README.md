Create a file named `.env` containing

```
ACME_EMAIL=your-email@example.com

HORNET_HOST=node.your-domain.com

DASHBOARD_USERNAME=admin
DASHBOARD_PASSWORD=0000000000000000000000000000000000000000000000000000000000000000
DASHBOARD_SALT=0000000000000000000000000000000000000000000000000000000000000000
```

Run `./prepare_docker.sh` to create the `data` folder with correct permissions.

You can generate your own dashboard password and salt by running:
`docker-compose run hornet tool pwd-hash`

Change the values accordingly, then run `docker-compose up -d`

You can check the logs using `docker-compose logs`

You will be able to access your node under:
https://node.your-domain.com/api/core/v2/info

Your dashboard under:
https://node.your-domain.com/dashboard

And grafana under:
https://node.your-domain.com/grafana

**Warning:** the initial grafana credentials are admin/admin, so be sure to log in once to change them.
