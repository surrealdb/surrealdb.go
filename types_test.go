package surrealdb_test

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/surrealdb/surrealdb.go"
	"github.com/test-go/testify/suite"
)

type TestTypesTestSuite struct {
	suite.Suite
}

func TestTypesSuite(t *testing.T) {
	suite.Run(t, new(TestTypesTestSuite))
}

func (suite *TestTypesTestSuite) SetupTest() {

}

func (suite *TestTypesTestSuite) TearDownSuite() {

}

func (suite *TestTypesTestSuite) Test_InvalidTokenString() {
	tokenData, err := surrealdb.TokenData{}.FromToken("")
	suite.Require().Error(err, surrealdb.ErrInvalidToken)
	suite.Require().Zero(tokenData)
}
func (suite *TestTypesTestSuite) Test_InvalidTokenSegments() {
	tokenData, err := surrealdb.TokenData{}.FromToken("ffff.bbbb")
	suite.Require().Error(err, surrealdb.ErrInvalidToken)
	suite.Require().Zero(tokenData)
}

func (suite *TestTypesTestSuite) Test_InvalidTokenBase64() {
	tokenData, err := surrealdb.TokenData{}.FromToken("ffff.bbbb.xxx")
	suite.Require().Error(err, surrealdb.ErrInvalidToken)
	suite.Require().Zero(tokenData)
}

func (suite *TestTypesTestSuite) Test_InvalidTokenJson() {
	invalid := base64.StdEncoding.EncodeToString([]byte("{pls, fail}"))
	tokenData, err := surrealdb.TokenData{}.FromToken(fmt.Sprintf("ffff.%s.xxx", invalid))
	suite.Require().Error(err, surrealdb.ErrInvalidToken)
	suite.Require().Zero(tokenData)
}

func (suite *TestTypesTestSuite) Test_ValidToken() {
	tokenData, err := surrealdb.TokenData{}.FromToken("eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzUxMiJ9.eyJpYXQiOjE2NjQwNjM5NzYsIm5iZiI6MTY2NDA2Mzk3NiwiZXhwIjoxNjY0MDY3NTc2LCJpc3MiOiJTdXJyZWFsREIiLCJucyI6InRlc3QiLCJkYiI6ImFwcGxpY2F0aW9uIiwic2MiOiJhY2NvdW50IiwiaWQiOiJ1c2VyOnl3amVhaG44Y3Y0a3JjeDI5a201In0.Fw5v2vBShVnMnNBKuuOjC24HWrMrBKMeADNgkEcebmjAJzpRIxPEEn5Ehr_a70Jnsl5xi7vl4-r5M2QxfODLZw")
	suite.Require().NoError(err, surrealdb.ErrInvalidToken)
	suite.Require().NotZero(tokenData)

	suite.Require().Equal("test", tokenData.Namespace)
	suite.Require().Equal("application", tokenData.Database)
	suite.Require().Equal("account", tokenData.Scope)
	suite.Require().Equal("user:ywjeahn8cv4krcx29km5", tokenData.Id)
	suite.Require().Equal("SurrealDB", tokenData.Issuer)
}
