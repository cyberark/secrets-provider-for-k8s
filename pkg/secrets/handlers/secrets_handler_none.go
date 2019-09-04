package handlers

type SecretHandlerNoneUseCase struct{}

func (secretHandlerNone SecretHandlerNoneUseCase) HandleSecrets() error {
	return nil
}
