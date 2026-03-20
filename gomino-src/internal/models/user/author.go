package user

type Author struct {
	Status                  int    `json:"status"`
	IsNicknameVerified      bool   `json:"isNicknameVerified"`
	UID                     string `json:"uid"`
	NdcID                   int    `json:"-"`
	Level                   int    `json:"level"`
	AccountMembershipStatus int    `json:"accountMembershipStatus"`
	MembershipStatus        *int   `json:"membershipStatus"`
	Reputation              int    `json:"reputation"`
	Role                    int    `json:"role"`
	Nickname                string `json:"nickname"`
	Icon                    string `json:"icon"`
}

type DetailedAuthor struct {
	Author
	FollowingStatus int    `json:"followingStatus"`
	IsGlobal        bool   `json:"isGlobal"`
	AminoID         string `json:"aminoId"`
	NdcID           int    `json:"ndcId"`
	MembersCount    int    `json:"membersCount"`
}

func (Author) TableName() string {
	return "users"
}

func (DetailedAuthor) TableName() string {
	return "users"
}
