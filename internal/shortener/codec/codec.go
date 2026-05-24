package codec

import (
	"time"

	"github.com/all-in-one/internal/shortener/model"
	shortenerv1 "github.com/all-in-one/internal/shortener/pb/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func ShortLinkToProto(link model.ShortLink) *shortenerv1.ShortLink {
	pb := &shortenerv1.ShortLink{
		Id:         link.ID,
		Code:       link.Code,
		TargetUrl:  link.TargetURL,
		IsActive:   link.IsActive,
		ClickCount: link.ClickCount,
		CreatedAt:  timestamppb.New(link.CreatedAt),
	}
	if link.ExpiresAt != nil {
		pb.ExpiresAt = timestamppb.New(*link.ExpiresAt)
	}
	if link.LastAccessedAt != nil {
		pb.LastAccessedAt = timestamppb.New(*link.LastAccessedAt)
	}
	return pb
}

func TimestampToTime(ts *timestamppb.Timestamp) *time.Time {
	if ts == nil {
		return nil
	}
	t := ts.AsTime()
	return &t
}
