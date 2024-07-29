package billitem

import "testing"

func Test_safeToInt(t *testing.T) {
	var ptr *int = new(int)
	*ptr = 1
	type args struct {
		value interface{}
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "Test safeToInt",
			args: args{
				value: 1,
			},
			want: 1,
		},
		{
			name: "nil value",
			args: args{
				value: nil,
			},
			want: 0,
		},
		{
			name: "int pointer",
			args: args{
				value: new(int),
			},
			want: 0,
		},
		{
			name: "int poiter with value",
			args: args{
				value: ptr,
			},
			want: 1,
		},
		{
			name: "float value",
			args: args{
				value: 1.1,
			},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := safeToInt(tt.args.value); got != tt.want {
				t.Errorf("safeToInt() = %v, want %v", got, tt.want)
			}
		})
	}
}
