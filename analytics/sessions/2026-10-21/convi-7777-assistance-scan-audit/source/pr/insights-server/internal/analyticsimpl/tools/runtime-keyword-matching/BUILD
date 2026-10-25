load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_binary(
    name = "runtime_keyword_matching",
    embed = [":runtime-keyword-matching_lib"],
    visibility = ["//visibility:public"],
)

go_library(
    name = "runtime-keyword-matching_lib",
    srcs = ["runtime_keyword_matching.go"],
    importpath = "github.com/cresta/go-servers/insights-server/internal/analyticsimpl/tools/runtime-keyword-matching",
    visibility = ["//visibility:private"],
    deps = [
        "//shared/utils",
        "@com_github_elastic_go_elasticsearch_v8//:go-elasticsearch",
        "@com_github_elastic_go_elasticsearch_v8//esapi",
    ],
)
