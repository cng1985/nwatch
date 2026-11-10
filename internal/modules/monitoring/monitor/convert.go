package monitor

import (
	"bytes"
	"errors"
	"io"

	"github.com/cng1985/nwatch/internal/modules/monitoring/shared"
	apperrors "github.com/cng1985/nwatch/internal/platform/errors"
)

// MonitorRequestToEntity 把业务请求应用到新实体。创建不接受客户端指定主键。
func MonitorRequestToEntity(request *MonitorRequest) *shared.Monitor {
	entity := &shared.Monitor{}
	request.Apply(entity)
	return entity
}

func newByteReader(data []byte) io.Reader {
	return bytes.NewReader(data)
}

// importError 把表格导入的业务错误映射为参数错误，其余保持原样。
func importError(err error) error {
	var rows *RowErrors
	if errors.As(err, &rows) {
		return apperrors.BadRequest(rows.Error())
	}
	var sheet *SheetError
	if errors.As(err, &sheet) {
		return apperrors.BadRequest(sheet.Error())
	}
	return err
}
