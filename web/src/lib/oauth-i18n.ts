// Copy for aio's login, signup and hand-off pages. English is aio's own
// wording; other languages are used only when an app sends users here and
// asks for one with ui_locales (see AuthRequestInfo.locale).

import { MIN_PASSWORD_LENGTH } from '#lib/password.js';

export type Locale = 'en' | 'id';

export interface AuthMessages {
	// shared
	securedBy: string;
	securedByDetail: (app: string) => string;
	username: string;
	password: string;
	// login
	loginTitle: string;
	loginTitleApp: (app: string) => string;
	loginSubtitle: string;
	loginSubtitleApp: string;
	usernamePlaceholder: string;
	passwordPlaceholder: string;
	loginButton: string;
	loggingIn: string;
	signUp: string;
	noAccount: string;
	credentialsRequired: string;
	invalidCredentials: string;
	accountBlocked: string;
	tooManyAttempts: string;
	loginFailed: string;
	loginError: string;
	// two-factor
	twoFactorTitle: string;
	enterRecoveryCode: string;
	enterTotpCode: string;
	recoveryCode: string;
	verificationCode: string;
	verify: string;
	verifying: string;
	useRecoveryCode: string;
	useAuthenticator: string;
	backToLogin: string;
	recoveryCodeRequired: string;
	verificationCodeRequired: string;
	tooManyCodeAttempts: string;
	invalidCode: string;
	// signup
	signupTitle: string;
	signupTitleApp: (app: string) => string;
	signupSubtitle: string;
	signupSubtitleApp: (app: string) => string;
	logIn: string;
	email: string;
	optional: string;
	chooseUsername: string;
	choosePassword: string;
	confirmPassword: string;
	reenterPassword: string;
	signUpButton: string;
	creatingAccount: string;
	haveAccount: string;
	usernamePasswordRequired: string;
	passwordsDontMatch: string;
	passwordTooShort: string;
	usernameTaken: string;
	tooManySignups: string;
	signupFailed: string;
	signupError: string;
	// hand-off (/oauth/login)
	handoffTitle: (app: string) => string;
	handoffDescription: (app: string) => string;
	handoffFallbackTitle: string;
	checkingSession: string;
	continuingTo: (app: string) => string;
	missingRequest: string;
	invalidLink: string;
	couldNotFinish: string;
	useAnotherAccount: string;
	goBack: string;
	// account address notice (RFC-003 §6.8)
	managedNotice: (app: string, host: string) => string;
	onlyHere: string;
	// forgot password
	forgotLink: string;
	forgotTitle: string;
	forgotSubtitle: string;
	usernameOrEmail: string;
	usernameOrEmailPlaceholder: string;
	sendTempPassword: string;
	sending: string;
	forgotSent: string;
	forgotSentDetail: string;
	noEmailTitle: string;
	noEmailText: (support: string) => string;
	noEmailNoSupport: string;
	emailAdmin: string;
	mailtoSubject: string;
	mailtoBody: (username: string) => string;
	emailResetUnavailable: string;
	backTo: (app: string) => string;
	somethingWrong: string;
	// choose a new password
	setPasswordTitle: string;
	setPasswordSubtitle: string;
	newPassword: string;
	saveAndContinue: string;
	saving: string;
	setPasswordFailed: string;
	// add email
	addEmailTitle: string;
	addEmailSubtitle: string;
	emailPlaceholder: string;
	sendConfirmationLink: string;
	later: string;
	confirmationSent: (email: string) => string;
	continueButton: string;
	invalidEmail: string;
	emailCantBeUsed: string;
	tooManyEmails: string;
	emailNotSent: string;
	// verify email
	verifyTitle: string;
	verifyingEmail: string;
	emailConfirmed: (email: string) => string;
	verifyFailed: string;
	goToAccount: string;
	// account page (/account, linked from apps)
	accountTitle: string;
	accountSubtitle: (app: string) => string;
	emailSection: string;
	emailVerified: string;
	emailNotVerified: string;
	noEmailYet: string;
	emailPendingNote: (email: string) => string;
	changeEmail: string;
	passwordSection: string;
	currentPassword: string;
	changePassword: string;
	passwordChanged: string;
	wrongCurrentPassword: string;
	emailUnavailableHere: string;
}

const en: AuthMessages = {
	securedBy: 'Login secured by All-in-one',
	securedByDetail: (app) => `One account for ${app} and other apps that use All-in-one.`,
	username: 'Username',
	password: 'Password',
	loginTitle: 'Login to your account',
	loginTitleApp: (app) => `Log in to ${app}`,
	loginSubtitle: 'Enter your email below to login to your account',
	loginSubtitleApp: 'Use your All-in-one account.',
	usernamePlaceholder: 'Enter your username',
	passwordPlaceholder: 'Your password',
	loginButton: 'Login',
	loggingIn: 'Logging in...',
	signUp: 'Sign Up',
	noAccount: "Don't have an account?",
	credentialsRequired: 'Username and password are required',
	invalidCredentials: 'Invalid username or password.',
	accountBlocked: 'This account is blocked.',
	tooManyAttempts: 'Too many attempts. Please try again later.',
	loginFailed: 'Login failed. Please check your credentials.',
	loginError: 'An error occurred during login. Please try again.',
	twoFactorTitle: 'Two-Factor Authentication',
	enterRecoveryCode: 'Enter one of your recovery codes',
	enterTotpCode: 'Enter the 6-digit code from your authenticator app',
	recoveryCode: 'Recovery Code',
	verificationCode: 'Verification Code',
	verify: 'Verify',
	verifying: 'Verifying...',
	useRecoveryCode: 'Use a recovery code',
	useAuthenticator: 'Use authenticator app instead',
	backToLogin: 'Back to login',
	recoveryCodeRequired: 'Recovery code is required',
	verificationCodeRequired: 'Verification code is required',
	tooManyCodeAttempts: 'Too many attempts. Please login again.',
	invalidCode: 'Invalid code. Please try again.',
	signupTitle: 'Create your account',
	signupTitleApp: (app) => `Create an account for ${app}`,
	signupSubtitle: 'Enter a username and password to get started',
	signupSubtitleApp: (app) => `It's an All-in-one account: you'll use it to log in to ${app}.`,
	logIn: 'Log In',
	email: 'Email',
	optional: '(optional)',
	chooseUsername: 'Choose a username',
	choosePassword: 'Choose a password',
	confirmPassword: 'Confirm Password',
	reenterPassword: 'Re-enter your password',
	signUpButton: 'Sign Up',
	creatingAccount: 'Creating account...',
	haveAccount: 'Already have an account?',
	usernamePasswordRequired: 'Username and password are required',
	passwordsDontMatch: 'Passwords do not match',
	passwordTooShort: `Password must be at least ${MIN_PASSWORD_LENGTH} characters long`,
	usernameTaken: 'Username already taken',
	tooManySignups: 'Too many sign-up attempts. Please try again later.',
	signupFailed: 'Failed to create account. Please try again.',
	signupError: 'An error occurred during sign up. Please try again.',
	handoffTitle: (app) => `Continue to ${app}`,
	handoffDescription: (app) => `${app} uses your all-in-one account to log you in.`,
	handoffFallbackTitle: 'Log in with All-in-one',
	checkingSession: 'Checking your all-in-one session…',
	continuingTo: (app) => `Continuing to ${app}…`,
	missingRequest: 'This login link is missing its request. Go back to the app and try again.',
	invalidLink: 'This login link is not valid.',
	couldNotFinish: 'Could not finish logging in.',
	useAnotherAccount: 'Use another account',
	goBack: 'Go back',
	managedNotice: (app, host) => `Your ${app} account is managed by All-in-one, so this page is on ${host}.`,
	onlyHere: 'Only enter your password on this address.',
	forgotLink: 'Forgot password?',
	forgotTitle: 'Forgot your password?',
	forgotSubtitle: "We'll email you a temporary password. Your current password keeps working until you choose a new one.",
	usernameOrEmail: 'Username or email',
	usernameOrEmailPlaceholder: 'Your username or email',
	sendTempPassword: 'Send temporary password',
	sending: 'Sending…',
	forgotSent: 'Check your email',
	forgotSentDetail:
		"If this account has a confirmed email, a temporary password is on its way. It works for 30 minutes. Nothing arrived? Check spam, or see below.",
	noEmailTitle: 'No email on your account?',
	noEmailText: (support) =>
		`Email the admin at ${support} from the address you want to use, with your username. The admin will check it's you and send a temporary password to that address.`,
	noEmailNoSupport: 'Contact the admin; they will check it is you and help you reset your password.',
	emailAdmin: 'Email the admin',
	mailtoSubject: 'Forgot my password',
	mailtoBody: (username) =>
		`Hi, I forgot my password.\n\nUsername: ${username}\n\nPlease send a temporary password to this email address.`,
	emailResetUnavailable: "Password reset by email isn't available yet.",
	backTo: (app) => `Back to ${app}`,
	somethingWrong: 'Something went wrong. Please try again.',
	setPasswordTitle: 'Choose a new password',
	setPasswordSubtitle: 'You logged in with a temporary password. Choose a new one to continue.',
	newPassword: 'New password',
	saveAndContinue: 'Save and continue',
	saving: 'Saving…',
	setPasswordFailed: 'Could not save your password. Please try again.',
	addEmailTitle: 'Add your email',
	addEmailSubtitle: 'So you can reset your password yourself if you ever forget it.',
	emailPlaceholder: 'you@example.com',
	sendConfirmationLink: 'Send confirmation link',
	later: 'Later',
	confirmationSent: (email) => `We sent a link to ${email}. Open it within 24 hours to confirm your email.`,
	continueButton: 'Continue',
	invalidEmail: 'Enter a valid email address.',
	emailCantBeUsed: "This email can't be used.",
	tooManyEmails: 'Too many confirmation emails today. Try again tomorrow.',
	emailNotSent: "The email couldn't be sent. Try again later.",
	verifyTitle: 'Confirm your email',
	verifyingEmail: 'Confirming…',
	emailConfirmed: (email) => `${email} is confirmed. You can now reset your password yourself if you forget it.`,
	verifyFailed: 'This link is invalid or has expired. Add your email again from your account settings.',
	goToAccount: 'Go to my account',
	accountTitle: 'Password & email',
	accountSubtitle: (app) => `For the account you use in ${app}.`,
	emailSection: 'Email',
	emailVerified: 'Confirmed',
	emailNotVerified: 'Not confirmed',
	noEmailYet: 'No email yet. Add one so you can reset your password yourself.',
	emailPendingNote: (email) => `Waiting for you to open the link sent to ${email}.`,
	changeEmail: 'Send confirmation link',
	passwordSection: 'Password',
	currentPassword: 'Current password',
	changePassword: 'Change password',
	passwordChanged: 'Password changed.',
	wrongCurrentPassword: 'Your current password is wrong.',
	emailUnavailableHere: 'Changing your email is not available yet; ask the admin.'
};

const id: AuthMessages = {
	securedBy: 'Login diamankan oleh All-in-one',
	securedByDetail: (app) => `Satu akun untuk ${app} dan aplikasi lain yang memakai All-in-one.`,
	username: 'Nama pengguna',
	password: 'Kata sandi',
	loginTitle: 'Masuk ke akun kamu',
	loginTitleApp: (app) => `Masuk ke ${app}`,
	loginSubtitle: 'Masukkan nama pengguna dan kata sandi kamu',
	loginSubtitleApp: 'Pakai akun All-in-one kamu.',
	usernamePlaceholder: 'Masukkan nama pengguna',
	passwordPlaceholder: 'Kata sandi kamu',
	loginButton: 'Masuk',
	loggingIn: 'Sedang masuk...',
	signUp: 'Daftar',
	noAccount: 'Belum punya akun?',
	credentialsRequired: 'Nama pengguna dan kata sandi wajib diisi.',
	invalidCredentials: 'Nama pengguna atau kata sandi salah.',
	accountBlocked: 'Akun ini diblokir.',
	tooManyAttempts: 'Terlalu banyak percobaan. Coba lagi nanti.',
	loginFailed: 'Gagal masuk. Periksa nama pengguna dan kata sandi kamu.',
	loginError: 'Terjadi kesalahan saat masuk. Coba lagi.',
	twoFactorTitle: 'Verifikasi dua langkah',
	enterRecoveryCode: 'Masukkan salah satu kode pemulihan kamu',
	enterTotpCode: 'Masukkan kode 6 digit dari aplikasi autentikator',
	recoveryCode: 'Kode pemulihan',
	verificationCode: 'Kode verifikasi',
	verify: 'Verifikasi',
	verifying: 'Memverifikasi...',
	useRecoveryCode: 'Pakai kode pemulihan',
	useAuthenticator: 'Pakai aplikasi autentikator',
	backToLogin: 'Kembali ke halaman masuk',
	recoveryCodeRequired: 'Kode pemulihan wajib diisi.',
	verificationCodeRequired: 'Kode verifikasi wajib diisi.',
	tooManyCodeAttempts: 'Terlalu banyak percobaan. Silakan masuk lagi.',
	invalidCode: 'Kode salah. Coba lagi.',
	signupTitle: 'Buat akun',
	signupTitleApp: (app) => `Buat akun untuk ${app}`,
	signupSubtitle: 'Pilih nama pengguna dan kata sandi untuk mulai',
	signupSubtitleApp: (app) => `Ini akun All-in-one: kamu memakainya untuk masuk ke ${app}.`,
	logIn: 'Masuk',
	email: 'Email',
	optional: '(opsional)',
	chooseUsername: 'Pilih nama pengguna',
	choosePassword: 'Pilih kata sandi',
	confirmPassword: 'Ulangi kata sandi',
	reenterPassword: 'Ketik ulang kata sandi',
	signUpButton: 'Daftar',
	creatingAccount: 'Membuat akun...',
	haveAccount: 'Sudah punya akun?',
	usernamePasswordRequired: 'Nama pengguna dan kata sandi wajib diisi.',
	passwordsDontMatch: 'Kata sandi tidak sama.',
	passwordTooShort: `Kata sandi minimal ${MIN_PASSWORD_LENGTH} karakter.`,
	usernameTaken: 'Nama pengguna sudah dipakai.',
	tooManySignups: 'Terlalu banyak percobaan daftar. Coba lagi nanti.',
	signupFailed: 'Gagal membuat akun. Coba lagi.',
	signupError: 'Terjadi kesalahan saat mendaftar. Coba lagi.',
	handoffTitle: (app) => `Lanjut ke ${app}`,
	handoffDescription: (app) => `${app} memakai akun All-in-one kamu untuk masuk.`,
	handoffFallbackTitle: 'Masuk dengan All-in-one',
	checkingSession: 'Memeriksa sesi All-in-one kamu…',
	continuingTo: (app) => `Melanjutkan ke ${app}…`,
	missingRequest: 'Tautan masuk ini tidak lengkap. Kembali ke aplikasi dan coba lagi.',
	invalidLink: 'Tautan masuk ini tidak valid.',
	couldNotFinish: 'Gagal menyelesaikan proses masuk.',
	useAnotherAccount: 'Pakai akun lain',
	goBack: 'Kembali',
	managedNotice: (app, host) => `Akun ${app} kamu dikelola oleh All-in-one, jadi halaman ini ada di ${host}.`,
	onlyHere: 'Masukkan kata sandi hanya di alamat ini.',
	forgotLink: 'Lupa kata sandi?',
	forgotTitle: 'Lupa kata sandi?',
	forgotSubtitle:
		'Kami kirim kata sandi sementara ke email kamu. Kata sandi lama tetap bisa dipakai sampai kamu membuat yang baru.',
	usernameOrEmail: 'Nama pengguna atau email',
	usernameOrEmailPlaceholder: 'Nama pengguna atau email kamu',
	sendTempPassword: 'Kirim kata sandi sementara',
	sending: 'Mengirim…',
	forgotSent: 'Cek email kamu',
	forgotSentDetail:
		'Jika akun ini punya email yang sudah dikonfirmasi, kata sandi sementara sedang dikirim. Berlaku 30 menit. Tidak masuk? Cek folder spam, atau lihat di bawah.',
	noEmailTitle: 'Akunmu belum punya email?',
	noEmailText: (support) =>
		`Kirim email ke admin di ${support} dari alamat yang ingin kamu pakai, sertakan nama pengguna kamu. Admin akan memastikan itu memang kamu, lalu mengirim kata sandi sementara ke alamat itu.`,
	noEmailNoSupport: 'Hubungi admin; admin akan memastikan itu memang kamu dan membantu mengatur ulang kata sandi.',
	emailAdmin: 'Kirim email ke admin',
	mailtoSubject: 'Lupa kata sandi',
	mailtoBody: (username) =>
		`Halo, saya lupa kata sandi.\n\nNama pengguna: ${username}\n\nMohon kirim kata sandi sementara ke alamat email ini.`,
	emailResetUnavailable: 'Atur ulang kata sandi lewat email belum tersedia.',
	backTo: (app) => `Kembali ke ${app}`,
	somethingWrong: 'Terjadi kesalahan. Coba lagi.',
	setPasswordTitle: 'Buat kata sandi baru',
	setPasswordSubtitle: 'Kamu masuk dengan kata sandi sementara. Buat kata sandi baru untuk melanjutkan.',
	newPassword: 'Kata sandi baru',
	saveAndContinue: 'Simpan dan lanjutkan',
	saving: 'Menyimpan…',
	setPasswordFailed: 'Gagal menyimpan kata sandi. Coba lagi.',
	addEmailTitle: 'Tambahkan email kamu',
	addEmailSubtitle: 'Supaya kamu bisa mengatur ulang kata sandi sendiri jika suatu saat lupa.',
	emailPlaceholder: 'kamu@contoh.com',
	sendConfirmationLink: 'Kirim tautan konfirmasi',
	later: 'Nanti saja',
	confirmationSent: (email) => `Kami mengirim tautan ke ${email}. Buka dalam 24 jam untuk mengonfirmasi email kamu.`,
	continueButton: 'Lanjutkan',
	invalidEmail: 'Masukkan alamat email yang valid.',
	emailCantBeUsed: 'Email ini tidak bisa dipakai.',
	tooManyEmails: 'Terlalu banyak email konfirmasi hari ini. Coba lagi besok.',
	emailNotSent: 'Email gagal dikirim. Coba lagi nanti.',
	verifyTitle: 'Konfirmasi email',
	verifyingEmail: 'Mengonfirmasi…',
	emailConfirmed: (email) => `${email} sudah dikonfirmasi. Sekarang kamu bisa mengatur ulang kata sandi sendiri jika lupa.`,
	verifyFailed: 'Tautan ini tidak valid atau sudah kedaluwarsa. Tambahkan email lagi dari pengaturan akun.',
	goToAccount: 'Ke akun saya',
	accountTitle: 'Kata sandi & email',
	accountSubtitle: (app) => `Untuk akun yang kamu pakai di ${app}.`,
	emailSection: 'Email',
	emailVerified: 'Terkonfirmasi',
	emailNotVerified: 'Belum dikonfirmasi',
	noEmailYet: 'Belum ada email. Tambahkan supaya kamu bisa mengatur ulang kata sandi sendiri.',
	emailPendingNote: (email) => `Menunggu kamu membuka tautan yang dikirim ke ${email}.`,
	changeEmail: 'Kirim tautan konfirmasi',
	passwordSection: 'Kata sandi',
	currentPassword: 'Kata sandi saat ini',
	changePassword: 'Ganti kata sandi',
	passwordChanged: 'Kata sandi sudah diganti.',
	wrongCurrentPassword: 'Kata sandi saat ini salah.',
	emailUnavailableHere: 'Mengganti email belum tersedia; hubungi admin.'
};

const catalogs: Record<Locale, AuthMessages> = { en, id };

export function authMessages(locale: string | undefined | null): AuthMessages {
	return catalogs[(locale as Locale) in catalogs ? (locale as Locale) : 'en'];
}
