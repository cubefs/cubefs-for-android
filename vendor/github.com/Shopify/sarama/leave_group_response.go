package sarama

type LeaveGroupResponse struct {
	Err KError
}

func (r *LeaveGroupResponse) encode(pe packetEncoder) error {
	pe.putInt16(int16(r.Err))
	return nil
}

func (r *LeaveGroupResponse) decode(pd packetDecoder, version int16) (err error) {
	kerr, err := pd.getInt16()
	if err != nil {
		return err
	}
	r.Err = KError(kerr)

	return nil
}

func (r *LeaveGroupResponse) key() int16 {
	return 13
}

func (r *LeaveGroupResponse) version() int16 {
	return 0
}

func (r *LeaveGroupResponse) headerVersion() int16 {
	return 0
}

func (r *LeaveGroupResponse) requiredVersion() KafkaVersion {
	return V0_9_0_0
}
