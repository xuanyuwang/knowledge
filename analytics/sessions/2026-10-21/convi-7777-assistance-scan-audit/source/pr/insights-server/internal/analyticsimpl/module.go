package analyticsimpl

import (
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	"github.com/cresta/shared-go/framework/action"
	"github.com/cresta/shared-go/framework/module"

	convertermodule "github.com/cresta/go-servers/shared/conversation/converter/module"
)

var Module = module.NewSimpleModule(func(binder module.Binder) error {
	if err := binder.Provide(Init); err != nil {
		return err
	}
	if err := binder.Provide(convertermodule.NewConverter); err != nil {
		return err
	}
	if err := action.BindGrpcServiceWithImpl(
		binder,
		&analyticspb.AnalyticsService_ServiceDesc,
		analyticspb.RegisterAnalyticsServiceHandler,
		func(impl *AnalyticsServiceImpl) *action.GrpcServiceImplEntry {
			return &action.GrpcServiceImplEntry{Impl: impl}
		},
	); err != nil {
		return err
	}

	return nil
})

var UserOutcomeModule = module.NewSimpleModule(func(binder module.Binder) error {
	return action.BindGrpcServiceWithImpl(
		binder,
		&analyticspb.UserOutcomeService_ServiceDesc,
		analyticspb.RegisterUserOutcomeServiceHandler,
		func(impl *AnalyticsServiceImpl) *action.GrpcServiceImplEntry {
			return &action.GrpcServiceImplEntry{
				Impl: &userOutcomeServiceWrapper{
					analyticsService: impl,
				},
			}
		},
	)
})
