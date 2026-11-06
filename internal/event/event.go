// Package event define os eventos de produto e o tipo da função que os registra. É neutro (como identity e apperr):
// as features chamam Emit sem conhecer quem guarda o evento, e o app liga a função ao ops.
//
// Um evento é só um nome da lista fechada abaixo, com a pessoa e o casal quando existem. Nunca leva texto.
package event

import "context"

const (
	UserRegistered            = "user_registered"
	HouseholdCreated          = "household_created"
	PartnerJoined             = "partner_joined"
	ModuleEnabled             = "module_enabled"
	ModuleDisabled            = "module_disabled"
	ReminderCreatedForPartner = "reminder_created_for_partner"
	PushEnabled               = "push_enabled"
	FeedbackSent              = "feedback_sent"
)

// Names é a lista fechada; a tabela product_events tem um CHECK com os mesmos nomes.
var Names = []string{
	UserRegistered, HouseholdCreated, PartnerJoined, ModuleEnabled, ModuleDisabled,
	ReminderCreatedForPartner, PushEnabled, FeedbackSent,
}

// Func registra um evento. userID e householdID podem ser vazios. Nunca deve falhar a operação de negócio
// (quem implementa só registra o erro no log) e roda fora da transação dela.
type Func func(ctx context.Context, name, userID, householdID string)

// Emit chama f se ela existir, para as features não precisarem checar nulo.
func (f Func) Emit(ctx context.Context, name, userID, householdID string) {
	if f != nil {
		f(ctx, name, userID, householdID)
	}
}
