# pgmig

> postgresql migration library &amp; tool

[![GoDoc][gd1]][gd2]
 [![codecov][cc1]][cc2]
 [![drone.io build status][bs1]][bs2]
 [![codebeat badge][cb1]][cb2]
 [![GoCard][gc1]][gc2]
 [![GitHub Release][gr1]][gr2]
 [![GitHub license][gl1]][gl2]

[cb1]: https://codebeat.co/badges/4f5f1a84-01ac-4c41-a257-62e2216f1428
[cb2]: https://codebeat.co/projects/github-com-pgmig-pgmig-master
[bs1]: https://cloud.drone.io/api/badges/pgmig/pgmig/status.svg
[bs2]: https://cloud.drone.io/pgmig/pgmig
[cc1]: https://codecov.io/gh/pgmig/pgmig/branch/master/graph/badge.svg
[cc2]: https://codecov.io/gh/pgmig/pgmig
[gd1]: https://godoc.org/github.com/pgmig/pgmig?status.svg
[gd2]: https://godoc.org/github.com/pgmig/pgmig
[gc1]: https://goreportcard.com/badge/github.com/pgmig/pgmig
[gc2]: https://goreportcard.com/report/github.com/pgmig/pgmig
[gr1]: https://img.shields.io/github/release/pgmig/pgmig.svg
[gr2]: https://github.com/pgmig/pgmig/releases
[gl1]: https://img.shields.io/github/license/pgmig/pgmig.svg
[gl2]: https://github.com/pgmig/pgmig/blob/master/LICENSE

This is an *alpha* state project. See [docs](https://pgmig.github.io/).

## Notes

### why pgx

In this lib, test results are received via `RAISE NOTICE` which is supported in pgx, but don't in sqlx, see [details](https://stackoverflow.com/a/59276504/5199825)

### why [go-imbed](https://github.com/growler/go-imbed)

Because I didn't find another lib which supports UnionFS, ie checks OS filesystem before embedded. If you know better, drop me a line please.

## TODO

* [ ] TODOs in code
* [ ] overwiew
* [ ] tests
* [ ] docs

## License

Apache License 2.0, see [LICENSE](LICENSE).

Copyright (c) 2019-2021 Aleksey Kovrizhkin <lekovr+pgmig@gmail.com>
